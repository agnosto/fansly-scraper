package download

import (
	//"bufio"
	"fmt"
	"io"
	"log"
    "bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
    "strconv"

	//"github.com/grafov/m3u8"

	//"go-fansly-scraper/config"
	"go-fansly-scraper/utils"
)

func GetM3U8Cookies(m3u8URL string) map[string]string {
    return map[string]string{
        "CloudFront-Key-Pair-Id": utils.GetQSValue(m3u8URL, "Key-Pair-Id"),
        "CloudFront-Policy":      utils.GetQSValue(m3u8URL, "Policy"),
        "CloudFront-Signature":   utils.GetQSValue(m3u8URL, "Signature"),
    }
}

func (d *Downloader) DownloadM3U8(modelName string, m3u8URL string, savePath string) error {
    cookies := GetM3U8Cookies(m3u8URL)
    baseURL, _ := utils.SplitURL(m3u8URL)

    // Fetch M3U8 playlist
    //log.Printf("Downloading M3U8 from URL: %s", m3u8URL)
    playlistContent, err := fetchM3U8Playlist(m3u8URL, cookies)
    //log.Printf("[DOWNLOAD M3U8] PlayList: %v", playlist)
    if err != nil {
        return err
    }

    //log.Printf("Playlist content:\n%s", playlistContent)
    segmentURLs, err := parseM3U8Playlist(playlistContent, baseURL, cookies)
    if err != nil {
        return err
    }

    // Download segments
    //log.Printf("Extracted segment URLs: %v", segmentURLs)
    segmentFiles, err := downloadSegments(segmentURLs, filepath.Dir(savePath), cookies)
    //log.Printf("[DOWNLOAD M3U8] SegmentedFiles: %v", segmentFiles)
    if err != nil {
        return err
    }

    // Combine segments using ffmpeg
    outputFile := filepath.Join(filepath.Dir(savePath), filepath.Base(savePath))
    err = combineSegments(segmentFiles, outputFile)
    //log.Printf("[DOWNLOAD M3U8] OutputFile: %v, or Error: %v", outputFile, err)
    if err != nil {
        return err
    }

    hashString, err := d.hashExistingFile(outputFile)
    if err != nil {
        return fmt.Errorf("error hashing M3U8 file: %w", err)
    }

    if err := d.saveFileHash(modelName, hashString, outputFile); err != nil {
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

	// Parse the M3U8 playlist
	//var segmentURLs []string
	//scanner := bufio.NewScanner(resp.Body)
	//for scanner.Scan() {
	//	line := strings.TrimSpace(scanner.Text())
	//	// Skip comments and empty lines
	//	if line == "" || strings.HasPrefix(line, "#") {
	//		continue
	//	}
	//	segmentURLs = append(segmentURLs, line)
	//}

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

func parseM3U8Playlist(content, baseURL string, cookies map[string]string) ([]string, error) {
    //var segmentURLs []string
    lines := strings.Split(content, "\n")
    var highestQualityURL string
    var highestBandwidth int

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

    if highestQualityURL != "" {
        fullURL := baseURL + "/" + highestQualityURL
        nestedContent, err := fetchM3U8Playlist(fullURL, cookies)
        if err != nil {
            return nil, err
        }
        return parseSegments(nestedContent, baseURL)
    }

    return parseSegments(content, baseURL)
}

func parseSegments(content, baseURL string) ([]string, error) {
    var segmentURLs []string
    lines := strings.Split(content, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if !strings.HasPrefix(line, "#") && strings.HasSuffix(line, ".ts") {
            if strings.HasPrefix(line, "http") {
                segmentURLs = append(segmentURLs, line)
            } else {
                segmentURLs = append(segmentURLs, baseURL+"/"+line)
            }
        }
    }
    return segmentURLs, nil
}

func downloadSegments(segmentURLs []string, savePath string, cookies map[string]string) ([]string, error) {
    var wg sync.WaitGroup
    segmentFiles := make([]string, len(segmentURLs))
    errors := make(chan error, len(segmentURLs))

    for i, segmentURL := range segmentURLs {
        wg.Add(1)
        go func(i int, segmentURL string) {
            defer wg.Done()
            fileName := filepath.Join(savePath, fmt.Sprintf("segment_%d.ts", i))
            
            err := downloadFile(segmentURL, fileName, cookies)
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
            }
            
            segmentFiles[i] = fileName
            //log.Printf("Successfully downloaded segment %d", i)
        }(i, segmentURL)
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        if err != nil {
            return nil, err
        }
    }

    return segmentFiles, nil
}

func downloadFile(url string, fileName string, cookies map[string]string) error {
    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }

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

    _, err = io.Copy(out, resp.Body)
    return err
}

func combineSegments(segmentFiles []string, outputFile string) error {
    tempFile, err := os.CreateTemp("", "segments_list_*.txt")
    if err != nil {
        return fmt.Errorf("failed to create temporary file: %w", err)
    }
    defer os.Remove(tempFile.Name())

    _, err = fmt.Fprint(tempFile, "ffconcat version 1.0\n")
    if err != nil {
        return fmt.Errorf("failed to write to temporary file: %w", err)
    }

    for _, file := range segmentFiles {
        // Use filepath.ToSlash to ensure consistent path separators
        _, err := fmt.Fprintf(tempFile, "file '%s'\n", filepath.ToSlash(file))
        if err != nil {
            return fmt.Errorf("failed to write to temporary file: %w", err)
        }
    }
    tempFile.Close()

    // Log the content of the temporary file
    //content, _ := os.ReadFile(tempFile.Name())
    //log.Printf("FFmpeg input file content:\n%s", string(content))

    args := []string{
        "-f", "concat",
        "-safe", "0",
        "-i", tempFile.Name(),
        "-c", "copy",
        "-f", "mpegts",  // Explicitly specify input format
        "-i", "pipe:0",  // Read from stdin
        outputFile,
    }
    cmd := exec.Command("ffmpeg", args...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err = cmd.Run()
    if err != nil {
        log.Printf("FFmpeg stdout: %s", stdout.String())
        log.Printf("FFmpeg stderr: %s", stderr.String())
        return fmt.Errorf("ffmpeg error: %v", err)
    }

    //log.Printf("FFmpeg command: %v", cmd.Args)
    return nil
}
