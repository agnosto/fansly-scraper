package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	//"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	githubAPIURL = "https://api.github.com/repos/agnosto/fansly-scraper/releases/latest"
)

type GithubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func CheckForUpdate(currentVersion string) error {
	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}

	if release.TagName == currentVersion {
		fmt.Println("You are already on the latest version.")
		return nil
	}

	fmt.Printf("New version available: %s\n", release.TagName)
	return updateBinary(release)
}

func getLatestRelease() (*GithubRelease, error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var release GithubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	return &release, nil
}

func updateBinary(release *GithubRelease) error {
	assetName := fmt.Sprintf("fansly-scraper_%s_%s_%s.tar.gz", strings.TrimPrefix(release.TagName, "v"), runtime.GOOS, runtime.GOARCH)

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no suitable binary found for your system")
	}

	fmt.Println("Downloading new version...")
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create a temporary directory in the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}
	tempDir := filepath.Join(currentDir, "tmp_update")
	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir) // Clean up the temporary directory when done

	tempFile := filepath.Join(tempDir, "update.tar.gz")
	outFile, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return err
	}

	// Extract the update
	err = extractUpdate(tempFile, tempDir)
	if err != nil {
		return err
	}

	// Find the new executable in the extracted files
	var newExePath string
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), "fansly-scraper") {
			newExePath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return err
	}

	if newExePath == "" {
		return fmt.Errorf("new executable not found in the update package")
	}

	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Rename the current executable to .old
	oldExePath := execPath + ".old"
	err = os.Rename(execPath, oldExePath)
	if err != nil {
		return err
	}

	// Move the new executable to replace the current one
	err = os.Rename(newExePath, execPath)
	if err != nil {
		// If moving the new executable fails, try to restore the old one
		os.Rename(oldExePath, execPath)
		return err
	}

	// Set the correct permissions for the new executable
	err = os.Chmod(execPath, 0755)
	if err != nil {
		return err
	}

	// Remove the old executable
	os.Remove(oldExePath)

	fmt.Println("Update successful. Please restart the application.")
	return nil
}

func extractUpdate(archivePath, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}
