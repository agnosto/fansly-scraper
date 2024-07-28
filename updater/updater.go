package updater

import (
    "archive/tar"
    "compress/gzip"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
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
    
    tempDir, err := os.MkdirTemp("", "fansly-scraper-update")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tempDir)
    
    gzr, err := gzip.NewReader(resp.Body)
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
        
        if header.Typeflag == tar.TypeReg {
            outPath := filepath.Join(tempDir, header.Name)
            outFile, err := os.Create(outPath)
            if err != nil {
                return err
            }
            if _, err := io.Copy(outFile, tr); err != nil {
                outFile.Close()
                return err
            }
            outFile.Close()
            
            if strings.HasPrefix(header.Name, "fansly-scraper") {
                execPath, err := os.Executable()
                if err != nil {
                    return err
                }
                
                err = os.Chmod(outPath, 0755)
                if err != nil {
                    return err
                }
                
                err = os.Rename(outPath, execPath)
                if err != nil {
                    return err
                }
                
                fmt.Println("Update successful. Please restart the application.")
                return nil
            }
        }
    }
    
    return fmt.Errorf("binary not found in the archive")
}
