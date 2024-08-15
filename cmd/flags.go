package cmd

import (
	"flag"
	"fmt"
	"github.com/agnosto/fansly-scraper/config"
	"os"
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

	flag.Usage = customUsage

	flag.StringVar(&flags.Username, "u", "", "Model username to download")
	flag.StringVar(&flags.Username, "username", "", "Model username to download")
	flag.StringVar(&flags.DownloadType, "d", "", "Download type: all, timeline, messages, or stories")
	flag.StringVar(&flags.DownloadType, "download", "", "Download type: all, timeline, messages, or stories")
	flag.BoolVar(&flags.Version, "v", false, "Display version information")
	flag.BoolVar(&flags.Version, "version", false, "Display version information")
	flag.StringVar(&flags.Monitor, "m", "", "Toggle monitoring for a model")
	flag.StringVar(&flags.Monitor, "monitor", "", "Toggle monitoring for a model")
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

	return flags, subcommand
}

func customUsage() {
	configPath := config.GetConfigPath()

	fmt.Fprintf(os.Stderr, "CLI Usage: fansly-scraper [options] [command]\n")
	fmt.Fprintf(os.Stderr, "Edit config located at %s to launch TUI\n\n", configPath)
	fmt.Fprintf(os.Stderr, "Options:\n")
	fmt.Fprintf(os.Stderr, "  -h, --help                Show this help message\n")
	fmt.Fprintf(os.Stderr, "  -u, --username=USERNAME   Model username to download\n")
	fmt.Fprintf(os.Stderr, "  -d, --download=TYPE       Download type: all, timeline, messages, or stories\n")
	fmt.Fprintf(os.Stderr, "  -v, --version             Display version information\n")
	fmt.Fprintf(os.Stderr, "  -m, --monitor=USERNAME    Toggle monitoring for a model\n")
	fmt.Fprintf(os.Stderr, "      --service=ACTION      Control the service: install, uninstall, start, stop, restart\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  update                    Check for and perform updates\n")
	fmt.Fprintf(os.Stderr, "  monitor [start|stop]      Start or stop the monitoring process\n")
}

func IsFFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func PrintUsage() {
	customUsage()
}
