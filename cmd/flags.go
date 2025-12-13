package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agnosto/fansly-scraper/config"
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
	WallID         string
	Limit          int
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

	flag.StringVar(&flags.WallID, "w", "", "Specific Wall ID to download from")
	flag.StringVar(&flags.WallID, "wall", "", "Specific Wall ID to download from")

	flag.StringVar(&flags.PostID, "p", "", "Download a specific post by ID or URL")
	flag.StringVar(&flags.PostID, "post", "", "Download a specific post by ID or URL")

	flag.IntVar(&flags.Limit, "l", 0, "Limit download to the x most recent items")
	flag.IntVar(&flags.Limit, "limit", 0, "Limit download to the x most recent items")

	flag.BoolVar(&flags.DumpChatLog, "dump-chat-log", false, "Export text chat history to a JSON file")

	flag.BoolVar(&flags.Version, "v", false, "Display version information")
	flag.BoolVar(&flags.Version, "version", false, "Display version information")

	flag.StringVar(&flags.Monitor, "m", "", "Toggle monitoring for a model")
	flag.StringVar(&flags.Monitor, "monitor", "", "Toggle monitoring for a model")

	flag.StringVar(&flags.Service, "service", "", "Control the service: install, uninstall, start, stop, restart")

	// Diagnosis flags
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
	appName := filepath.Base(os.Args[0])

	// Header
	fmt.Fprintf(os.Stderr, "\nUsage: %s [options] [command]\n", appName)
	fmt.Fprintf(os.Stderr, "Configuration: %s\n", configPath)
	fmt.Fprintf(os.Stderr, "Run without arguments to launch the Interactive TUI.\n\n")

	// Scraping Options
	fmt.Fprintf(os.Stderr, "Scraping Options:\n")
	fmt.Fprintf(os.Stderr, "  -u, --username <user>     Model username to download.\n")
	fmt.Fprintf(os.Stderr, "  -d, --download <type>     Content type: 'all', 'timeline', 'messages', 'stories'.\n")
	fmt.Fprintf(os.Stderr, "                            (Defaults to 'all', or 'timeline' if --wall is set)\n")
	fmt.Fprintf(os.Stderr, "                            ('all' includes profile pics if enabled in config)\n")
	fmt.Fprintf(os.Stderr, "  -w, --wall <id>           Specific Wall ID to download (filters timeline).\n")
	fmt.Fprintf(os.Stderr, "  -p, --post <id|url>       Download a specific post by ID or URL.\n")
	fmt.Fprintf(os.Stderr, "  -l, --limit <num>         Limit download to the x most recent items.\n")
	fmt.Fprintf(os.Stderr, "      --dump-chat-log       Export text chat history to a JSON file (requires -u).\n\n")

	// Operational Options
	fmt.Fprintf(os.Stderr, "Operational Options:\n")
	fmt.Fprintf(os.Stderr, "  -m, --monitor <user>      Toggle background monitoring for a model's stream.\n")
	fmt.Fprintf(os.Stderr, "      --service <action>    Control service: install, uninstall, start, stop, restart (not yet implemented).\n")
	fmt.Fprintf(os.Stderr, "  -v, --version             Display version information.\n")
	fmt.Fprintf(os.Stderr, "  -h, --help                Show this help message.\n\n")

	// Diagnosis Options
	fmt.Fprintf(os.Stderr, "Diagnosis & Debugging:\n")
	fmt.Fprintf(os.Stderr, "  --diagnosis               Run the diagnosis suite to check config/API.\n")
	fmt.Fprintf(os.Stderr, "      --level <1-3>         Verbosity level (default: 1).\n")
	fmt.Fprintf(os.Stderr, "      --creator <user>      Run specific tests on a creator.\n")
	fmt.Fprintf(os.Stderr, "      --postID <id>         Run specific tests on a post ID.\n")
	fmt.Fprintf(os.Stderr, "      --output <file>       Path for the report file.\n")
	fmt.Fprintf(os.Stderr, "      --keep-tmp            Don't delete temp files after diagnosis.\n\n")

	// Commands
	fmt.Fprintf(os.Stderr, "Available Commands:\n")
	fmt.Fprintf(os.Stderr, "  update                    Check for and perform updates.\n")
	fmt.Fprintf(os.Stderr, "  monitor [start|stop]      Manually control the monitoring process.\n\n")

	// Examples
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  # Download everything (Timeline, Messages, Stories) for a user:\n")
	fmt.Fprintf(os.Stderr, "  %s -u username\n\n", appName)

	fmt.Fprintf(os.Stderr, "  # Download only messages:\n")
	fmt.Fprintf(os.Stderr, "  %s -u username -d messages\n\n", appName)

	fmt.Fprintf(os.Stderr, "  # Download a specific Wall (timeline only):\n")
	fmt.Fprintf(os.Stderr, "  %s -u username --wall 793928278367805440\n\n", appName)

	fmt.Fprintf(os.Stderr, "  # Download a Wall AND messages/stories:\n")
	fmt.Fprintf(os.Stderr, "  %s -u username --wall 793928278367805440 -d all\n\n", appName)

	fmt.Fprintf(os.Stderr, "  # Download a single post:\n")
	fmt.Fprintf(os.Stderr, "  %s -p 837364635748286464\n\n", appName)

	fmt.Fprintf(os.Stderr, "  # Toggle monitoring for a user:\n")
	fmt.Fprintf(os.Stderr, "  %s -m username\n", appName)
}

func IsFFmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func PrintUsage() {
	customUsage()
}
