package updater

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    //"path/filepath"
    "runtime"
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
    assetName := fmt.Sprintf("fansly-scraper-%s-%s", runtime.GOOS, runtime.GOARCH)
    if runtime.GOOS == "windows" {
        assetName += ".exe"
    }

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

    execPath, err := os.Executable()
    if err != nil {
        return err
    }

    tempPath := execPath + ".new"
    out, err := os.Create(tempPath)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    if err != nil {
        return err
    }

    err = os.Chmod(tempPath, 0755)
    if err != nil {
        return err
    }

    err = os.Rename(tempPath, execPath)
    if err != nil {
        return err
    }

    fmt.Println("Update successful. Please restart the application.")
    return nil
}
