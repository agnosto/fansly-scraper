package download

import (
	//"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	//"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"context"
	"golang.org/x/sync/semaphore"
	//"github.com/grafov/m3u8"

	//"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/utils"
)

var m3u8Semaphore = semaphore.NewWeighted(2) // Limit to 2 concurrent M3U8 downloads, shitop programming

func GetM3U8Cookies(m3u8URL string) map[string]string {
	return map[string]string{
		"CloudFront-Key-Pair-Id": utils.GetQSValue(m3u8URL, "Key-Pair-Id"),
		"CloudFront-Policy":      utils.GetQSValue(m3u8URL, "Policy"),
		"CloudFront-Signature":   utils.GetQSValue(m3u8URL, "Signature"),
	}
}

func (d *Downloader) DownloadM3U8(ctx context.Context, modelName string, m3u8URL string, savePath string, postID string) error {
	fileType := "video"

	if err := m3u8Semaphore.Acquire(ctx, 1); err != nil {
		return err
	}
	defer m3u8Semaphore.Release(1)
	cookies := GetM3U8Cookies(m3u8URL)
	//baseURL, _ := utils.SplitURL(m3u8URL)

	// Fetch M3U8 playlist
	//log.Printf("Downloading M3U8 from URL: %s", m3u8URL)
	playlistContent, err := fetchM3U8Playlist(m3u8URL, cookies)
	//log.Printf("[DOWNLOAD M3U8] PlayList: %v", playlist)
	if err != nil {
		return err
	}

	//log.Printf("Playlist content:\n%s", playlistContent)
	segmentURLs, err := parseM3U8Playlist(playlistContent, m3u8URL, cookies)
	if err != nil {
		return err
	}

	segmentDir := filepath.Join(filepath.Dir(savePath), "segments_"+postID)
	if err := os.MkdirAll(segmentDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create segment directory: %w", err)
	}

	// Download segments
	//log.Printf("Extracted segment URLs: %v", segmentURLs)
	segmentFiles, err := downloadSegments(ctx, segmentURLs, segmentDir, cookies)
	//log.Printf("[DOWNLOAD M3U8] SegmentedFiles: %v", segmentFiles)
	if err != nil {
		return err
	}

	// Combine segments using ffmpeg
	outputFile := filepath.Join(filepath.Dir(savePath), filepath.Base(savePath))
	err = combineSegments(segmentFiles, outputFile, segmentDir)
	//log.Printf("[DOWNLOAD M3U8] OutputFile: %v, or Error: %v", outputFile, err)
	if err != nil {
		return err
	}

	hashString, err := d.hashExistingFile(outputFile)
	if err != nil {
		return fmt.Errorf("error hashing M3U8 file: %w", err)
	}

	if err := d.saveFileHash(modelName, hashString, outputFile, fileType); err != nil {
		return fmt.Errorf("error saving hash for M3U8 file: %w", err)
	}

	// Clean up segment files
	for _, file := range segmentFiles {
		os.Remove(file)
	}

	return nil
}

func fetchM3U8Playlist(m3u8URL string, cookies map[string]string) (string, error) {
	req, err := http.NewRequest("GET", m3u8URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add cookies to the request
	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	// Send the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch M3U8 playlist: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading M3U8 playlist: %w", err)
	}

	// Check for errors during scanning
	//if err := scanner.Err(); err != nil {
	//	return "", fmt.Errorf("error reading M3U8 playlist: %w", err)
	//}

	content := string(bodyBytes)
	//log.Printf("Raw M3U8 content:\n%s", content)

	return content, nil
}

func parseM3U8Playlist(content, m3u8URL string, cookies map[string]string) ([]string, error) {
	baseURL, err := url.Parse(m3u8URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse m3u8 URL: %w", err)
	}

	lines := strings.Split(content, "\n")
	var highestQualityURL string
	var highestBandwidth int

	// Check if this is a master playlist
	isMasterPlaylist := false
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			isMasterPlaylist = true
			break
		}
	}

	//log.Printf("Is master playlist: %v", isMasterPlaylist)

	if isMasterPlaylist {
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
				bandwidthStr := strings.Split(strings.Split(line, "BANDWIDTH=")[1], ",")[0]
				bandwidth, _ := strconv.Atoi(bandwidthStr)
				if bandwidth > highestBandwidth {
					highestBandwidth = bandwidth
					highestQualityURL = strings.TrimSpace(lines[i+1])
				}
			}
		}

		logger.Logger.Printf("Highest quality URL: %s", highestQualityURL)

		if highestQualityURL != "" {
			// Construct the full URL for the highest quality stream
			newURL, err := url.Parse(highestQualityURL)
			if err != nil {
				return nil, fmt.Errorf("failed to parse highest quality URL: %w", err)
			}
			newURL = baseURL.ResolveReference(newURL)

			// Preserve query parameters
			newURL.RawQuery = baseURL.RawQuery

			logger.Logger.Printf("Fetching media playlist from: %s", newURL.String())

			// Fetch the media playlist
			nestedContent, err := fetchM3U8Playlist(newURL.String(), cookies)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch media playlist: %w", err)
			}

			//logger.Logger.Printf("Media playlist content:\n%s", nestedContent)

			return parseM3U8Playlist(nestedContent, newURL.String(), cookies)
		}
	}

	// If it's not a master playlist or we couldn't find a higher quality stream,
	// parse the segments directly
	//log.Printf("Parsing segments directly")
	return parseSegments(content, m3u8URL)
}

func parseSegments(content, baseURL string) ([]string, error) {
	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	var segmentURLs []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") && (strings.HasSuffix(line, ".ts") || strings.HasSuffix(line, ".m3u8") || strings.HasSuffix(line, ".m4s")) {
			segmentURL, err := url.Parse(line)
			if err != nil {
				return nil, fmt.Errorf("failed to parse segment URL: %w", err)
			}

			if !segmentURL.IsAbs() {
				segmentURL = baseURLParsed.ResolveReference(segmentURL)
			}

			// Preserve query parameters
			if segmentURL.RawQuery == "" {
				segmentURL.RawQuery = baseURLParsed.RawQuery
			}

			segmentURLs = append(segmentURLs, segmentURL.String())
		}
	}

	//log.Printf("Found %d segment URLs", len(segmentURLs))
	for i, url := range segmentURLs {
		logger.Logger.Printf("Segment %d: %s", i, url)
	}

	return segmentURLs, nil
}

func downloadSegments(ctx context.Context, segmentURLs []string, savePath string, cookies map[string]string) ([]string, error) {
	var wg sync.WaitGroup
	segmentFiles := make([]string, len(segmentURLs))
	errors := make(chan error, len(segmentURLs))

	sem := semaphore.NewWeighted(3)

	//log.Printf("Attempting to download %d segments", len(segmentURLs))

	for i, segmentURL := range segmentURLs {
		wg.Add(1)
		go func(i int, segmentURL string) {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				errors <- fmt.Errorf("failed to acquire semaphore: %w", err)
				return
			}
			defer sem.Release(1)

			fileName := filepath.Join(savePath, fmt.Sprintf("segment_%d.ts", i))

			err := downloadFile(ctx, segmentURL, fileName, cookies)
			if err != nil {
				log.Printf("Error downloading segment %d: %v", i, err)
				errors <- err
				return
			}

			// Verify file size
			fileInfo, err := os.Stat(fileName)
			if err != nil {
				log.Printf("Error checking file size for segment %d: %v", i, err)
				errors <- err
				return
			}
			if fileInfo.Size() == 0 {
				log.Printf("Warning: Segment %d has zero size", i)
				errors <- fmt.Errorf("segment %d has zero size", i)
				return
			}

			segmentFiles[i] = fileName
			//log.Printf("Successfully downloaded segment %d: %s", i, fileName)
		}(i, segmentURL)
	}

	wg.Wait()
	close(errors)

	var errs []error
	for err := range errors {
		if err != nil {
			errs = append(errs, err)
		}
	}

	// Check if all segments were downloaded successfully
	successfulDownloads := 0
	for i, file := range segmentFiles {
		if file == "" {
			errs = append(errs, fmt.Errorf("segment %d failed to download", i))
		} else {
			successfulDownloads++
		}
	}

	//log.Printf("Successfully downloaded %d out of %d segments", successfulDownloads, len(segmentURLs))

	if len(errs) > 0 {
		return nil, fmt.Errorf("multiple errors occurred: %v", errs)
	}

	return segmentFiles, nil
}

func downloadFile(ctx context.Context, url string, fileName string, cookies map[string]string) error {
	logger.Logger.Printf("Downloading file: %s to %s", url, fileName)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	for k, v := range cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.Logger.Printf("Successfully downloaded %s (%d bytes)", fileName, written)
	return nil
}

func combineSegments(segmentFiles []string, outputFile string, segmentDir string) error {
	tempFile, err := os.CreateTemp(segmentDir, "segments_list_*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	_, err = fmt.Fprint(tempFile, "ffconcat version 1.0\n")
	if err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	for _, file := range segmentFiles {
		absPath, err := filepath.Abs(file)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		_, err = fmt.Fprintf(tempFile, "file '%s'\n", filepath.ToSlash(absPath))
		if err != nil {
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}
	}
	tempFile.Close()

	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", tempFile.Name(),
		"-c", "copy",
		"-f", "mpegts",
		"-i", "pipe:0",
		outputFile,
	}
	cmd := exec.Command("ffmpeg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logger.Logger.Printf("FFmpeg stdout: %s", stdout.String())
		logger.Logger.Printf("FFmpeg stderr: %s", stderr.String())
		return fmt.Errorf("ffmpeg error: %v", err)
	}

	// Clean up segment files
	for _, file := range segmentFiles {
		if err := os.Remove(file); err != nil {
			logger.Logger.Printf("Failed to remove segment file %s: %v", file, err)
		}
	}

	// Remove the segment directory
	if err := os.RemoveAll(segmentDir); err != nil {
		logger.Logger.Printf("Failed to remove segment directory %s: %v", segmentDir, err)
	}

	return nil
}
