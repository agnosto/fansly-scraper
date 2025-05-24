# Fansly Scraper

A simple all in one tool to download and monitor content from Fansly creators.

> **⚠️ Currently in development** - Some features may not work perfectly. See [known issues](#known-issues) below.


## Requirements

- **Optional but highly recommended**: [ffmpeg](https://ffmpeg.org/) for livestream recording and saving higher quality videos.
- **Optional**: [mt](https://github.com/mutschler/mt) for better contact sheets

## Quick Start

### 1. Download
- **Easy way**: Visit the [download page](https://agnosto.github.io/projects/fansly-scraper/) (auto-detects your system)
- **Manual way**: Get from [GitHub releases](https://github.com/agnosto/fansly-scraper/releases)
- **Intall Via Go**: 
```bash
go install github.com/agnosto/fansly-scraper/cmd/fansly-scraper@latest
```

### 2. Run the Program
```bash
./fansly-scraper
```

The program will create a [config](./config.md#location) file for you to edit on first run.

## Basic Usage

### Interactive Mode (Recommended for beginners)
```bash
./fansly-scraper
```

### Command Line Mode
```bash
# Download all content from a creator
./fansly-scraper -u {creator-name}

# Download specific content types
./fansly-scraper -u {creator-name} -d [all|timeline|messages|stories]

# Monitor for live streams
./fansly-scraper -m {creator-name}

# Start/stop monitoring
./fansly-scraper monitor [start|stop]
```

**Note**: Live monitoring requires keeping your terminal/shell session active. Consider using [tmux](https://github.com/tmux/tmux/wiki) or [zellij](https://github.com/zellij-org/zellij) on linux/wsl to run it in the background.

### Update the Program
```bash
./fansly-scraper update
```

## Advanced Setup

- **Configuration options**: See [config.md](./config.md)
- **Development roadmap**: See [TRACKER.md](./TRACKER.md)
- **Build from source**:
  ```bash
  git clone https://github.com/agnosto/fansly-scraper && cd fansly-scraper
  go build -v -ldflags "-w -s" -o fansly-scraper ./cmd/fansly-scraper
  ```

## Known Issues

- **Chat recording**: May occasionally miss messages during reconnections
- **MacOS users**: May need to allow notifications in [system settings](https://github.com/gen2brain/beeep/issues/67#issuecomment-2646474049)
- **Live status**: Press `r` in TUI to refresh live status
- **Duplicate files**: Sometimes the same image may appear twice due to Fansly's API structure

## Support the Project

If this tool helps you, consider donating:

<table>
  <tr>
    <td align="center"><strong>Bitcoin (BTC)</strong></td>
    <td align="center"><strong>Solana (SOL)</strong></td>
  </tr>
  <tr>
    <td align="center">
      <img src="./assets/btc_qr.png" alt="Bitcoin QR Code" width="200"/>
      <p><code>bc1q0e78wrtc9ezp6tqv000wfewgqf2ue4tpzdk7ee</code></p>
    </td>
    <td align="center">
      <img src="./assets/sol_qr.png" alt="Solana QR Code" width="200"/>
      <p><code>Bv3kYZcwSTHXAQtnPddTF27D3F6Gc29v2MfFLqmGF6Gf</code></p>
    </td>
  </tr>
</table>

---

> [!CAUTION]
> **Disclaimer**: This tool is not affiliated with Fansly or Select Media LLC. Use at your own risk. The developer of this script is not responsible for the end users' actions or any outcomes that may be taken upon the end users' account. Use at your own risk.

