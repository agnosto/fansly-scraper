package cmd

import (
	"flag"
	"os/exec"
)

type Flags struct {
	Username     string
	DownloadType string
}

func ParseFlags() (Flags, string) {
	username := flag.String("u", "", "Model username to download")
	flag.StringVar(username, "username", "", "Model username to download")
	downloadType := flag.String("d", "", "Download type: all, timeline, messages, or stories")
	flag.StringVar(downloadType, "download", "", "Download type: all, timeline, messages, or stories")

	flag.Parse()
    args := flag.Args()

    if len(args) > 0 {
		return Flags{}, args[0]
	}

	return Flags{
		Username:     *username,
		DownloadType: *downloadType,
	}, ""
}

func IsFFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
