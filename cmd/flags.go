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
	PostID         string
	RunDiagnosis   bool
	DiagnosisFlags DiagnosisFlags
	DumpChatLog    bool
}

type DiagnosisFlags struct {
	Level      int
	OutputFile string
	Creator    string
	PostID     string
	KeepTmp    bool
}

func ParseFlags() (Flags, string) {
	flags := Flags{}

	flag.Usage = customUsage

	// Main operational flags
	flag.StringVar(&flags.Username, "u", "", "Model username to download")
	flag.StringVar(&flags.Username, "username", "", "Model username to download")
	flag.StringVar(&flags.DownloadType, "d", "", "Download type: all, timeline, messages, or stories")
	flag.StringVar(&flags.DownloadType, "download", "", "Download type: all, timeline, messages, or stories")
	flag.BoolVar(&flags.Version, "v", false, "Display version information")
	flag.BoolVar(&flags.Version, "version", false, "Display version information")
	flag.StringVar(&flags.Monitor, "m", "", "Toggle monitoring for a model")
	flag.StringVar(&flags.Monitor, "monitor", "", "Toggle monitoring for a model")
	flag.StringVar(&flags.Service, "service", "", "Control the service: install, uninstall, start, stop, restart")
	flag.StringVar(&flags.PostID, "p", "", "Download a specific post by ID or URL")
	flag.StringVar(&flags.PostID, "post", "", "Download a specific post by ID or URL")
	flag.BoolVar(&flags.DumpChatLog, "dump-chat-log", false, "Export text chat history to a JSON file")

	// Diagnosis flags, now grouped logically
	flag.BoolVar(&flags.RunDiagnosis, "diagnosis", false, "Run the diagnosis suite")
	flag.IntVar(&flags.DiagnosisFlags.Level, "level", 1, "Verbosity level for diagnosis (1-3)")
	flag.StringVar(&flags.DiagnosisFlags.OutputFile, "output", "", "Output file for diagnosis report")
	flag.StringVar(&flags.DiagnosisFlags.Creator, "creator", "", "Specify a creator's username for targeted tests")
	flag.StringVar(&flags.DiagnosisFlags.PostID, "postID", "", "Specify a post ID for targeted tests")
	flag.BoolVar(&flags.DiagnosisFlags.KeepTmp, "keep-tmp", false, "Do not delete the temporary download directory after diagnosis")

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

	// Grouping User/Download options together
	fmt.Fprintf(os.Stderr, "  -u, --username=USERNAME   Model username to download\n")
	fmt.Fprintf(os.Stderr, "  -d, --download=TYPE       Download type: all, timeline, messages, or stories\n")
	fmt.Fprintf(os.Stderr, "      --dump-chat-log       Export text chat history to a JSON file (requires -u)\n")
	fmt.Fprintf(os.Stderr, "  -p, --post=POST_ID/URL    Download a specific post by ID or URL\n")

	// Operational flags
	fmt.Fprintf(os.Stderr, "  -m, --monitor=USERNAME    Toggle monitoring for a model\n")
	fmt.Fprintf(os.Stderr, "      --service=ACTION      Control the service: install, uninstall, start, stop, restart\n")
	fmt.Fprintf(os.Stderr, "  -v, --version             Display version information\n\n")

	fmt.Fprintf(os.Stderr, "Diagnosis:\n")
	fmt.Fprintf(os.Stderr, "  --diagnosis               Run the diagnosis suite with the following options:\n")
	fmt.Fprintf(os.Stderr, "      --level=LEVEL         Verbosity level (1-3, default: 1)\n")
	fmt.Fprintf(os.Stderr, "      --output=FILE         Output file for the report (default: diagnosis-report-YYYY-MM-DD_HH-MM-SS.txt)\n")
	fmt.Fprintf(os.Stderr, "      --creator=USERNAME    Run tests on a specific creator\n")
	fmt.Fprintf(os.Stderr, "      --postID=ID           Run tests on a specific post (requires --creator)\n")
	fmt.Fprintf(os.Stderr, "      --keep-tmp            Do not delete the temporary download directory after diagnosis\n\n")

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
