package cmd

import (
	"flag"
	"os/exec"
)

type Flags struct {
	Username       string
	DownloadType   string
	Version        bool
	Monitor        string
	Service        string
	MonitorCommand string
}

func ParseFlags() (Flags, string) {
	flags := Flags{}

	flag.StringVar(&flags.Username, "u", "", "Model username to download")
	flag.StringVar(&flags.Username, "username", "", "Model username to download")
	flag.StringVar(&flags.DownloadType, "d", "", "Download type: all, timeline, messages, or stories")
	flag.StringVar(&flags.DownloadType, "download", "", "Download type: all, timeline, messages, or stories")
	flag.BoolVar(&flags.Version, "v", false, "Display version information")
	flag.BoolVar(&flags.Version, "version", false, "Display version information")
	flag.StringVar(&flags.Monitor, "monitor", "", "Toggle monitoring for a model")
	flag.StringVar(&flags.Monitor, "m", "", "Toggle monitoring for a model (shorthand)")
	flag.StringVar(&flags.Service, "service", "", "Control the service: install, uninstall, start, stop, restart")

	flag.Parse()

	args := flag.Args()
	var subcommand string

	if len(args) > 0 {
		subcommand = args[0]
		if subcommand == "monitor" && len(args) > 1 {
			flags.MonitorCommand = args[1]
		}
	}

	/*return Flags{
		Username:       *username,
		DownloadType:   *downloadType,
		Version:        *version,
		Monitor:        *monitor,
		Service:        *service,
		MonitorCommand: monitorCommand,
	}, ""*/
	return flags, subcommand
}

func IsFFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
