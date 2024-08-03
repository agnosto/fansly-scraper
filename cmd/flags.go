package cmd

import (
	"flag"
	"os/exec"
)

type Flags struct {
	Username     string
	DownloadType string
    Version      bool 
    Monitor      string
    Service      string
}

func ParseFlags() (Flags, string) {
	username := flag.String("u", "", "Model username to download")
	flag.StringVar(username, "username", "", "Model username to download")
	downloadType := flag.String("d", "", "Download type: all, timeline, messages, or stories")
	flag.StringVar(downloadType, "download", "", "Download type: all, timeline, messages, or stories")
    version := flag.Bool("v", false, "Display version information")
    flag.BoolVar(version, "version", false, "Display version information")
    monitor := flag.String("monitor", "", "Toggle monitoring for a model")
    flag.StringVar(monitor, "m", "", "Toggle monitoring for a model (shorthand)")

    service := flag.String("service", "", "Control the service: install, uninstall, start, stop, restart")

	flag.Parse()
    args := flag.Args()

    if len(args) > 0 {
		return Flags{}, args[0]
	}

	return Flags{
		Username:     *username,
		DownloadType: *downloadType,
        Version:      *version,
        Monitor:      *monitor,
        Service:      *service,
	}, ""
}

func IsFFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
