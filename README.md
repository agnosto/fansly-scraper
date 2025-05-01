# Fansly Scraper 

> [!NOTE] 
> This is currently under development, everything is still being planned out and tested. Feel free to create issues/pr's with any features/bugs to assist in the creation of this project.
> [Tracker](./TRACKER.md) will hold a list of things to do for the project, as well as act as a kind of roadmap.
> I also recommend reading [Program Status](https://github.com/agnosto/fansly-scraper?tab=readme-ov-file#program-status) for more info and currently known issues before running.


## A simple all in one fansly interaction tool.

The program will automatically move/download an example config into the config path from either the current directory or the github repo respectively, you will need to edit the config to run the program.

> [!IMPORTANT]
> [ffmpeg](https://ffmpeg.org/) is not needed for content saving, but for livestream recording and potentially saving higher quality videos, it is required.
> [mt](https://github.com/mutschler/mt) is recommended for improved contact sheet generation, be sure to have it installed to your PATH. 

## Installing the program

### Releases

Pre-compiled binaries can be downloaded from the [releases](https://github.com/agnosto/fansly-scraper/releases) section.


### Manual Compile

```bash
git clone https://github.com/agnosto/fansly-scraper && cd fansly-scraper 

go build -v -ldflags "-w -s" -o fansly-scraper ./cmd/fansly-scraper

# run the binary
./fansly-scraper
```

### Install Via Go

```bash
go install github.com/agnosto/fansly-scraper/cmd/fansly-scraper@latest
```

## Config

To learn about the different config options you have available, refer to [config](./config.md).

## Running the program 

### Interactive TUI 

Simply run the program to launch the tui:

```bash
./fansly-scraper
```

### Non-Interactive  CLI Mode 

```bash 
# Defaults to selecting all
./fansly-scraper --username {creator name} 
# Or using short flag 
./fansly-scraper -u {creator name}

# With Download Option 
./fansly-scraper --username {creator name} --download [all|timeline|messages|stories]
# Or using short flags
./fansly-scraper -u {creator name} -d [all|timeline|messages|stories]

# Add Models to Live Monitoring 
./fansly-scraper --monitor {creator name}
#Or with short flags
./fansly-scraper -m {creator name}

# Live Monitoring Control
./fansly-scraper monitor [start|stop]
```
> [!NOTE]
> Live monitoring requires an active running shell/terminal session, you can use something like [zellij](https://github.com/zellij-org/zellij)/[tmux](https://github.com/tmux/tmux/wiki). I may go back to the idea of having it be a background service, but for the time being just implemented as a go version of my [fansly-recorder](https://github.com/agnosto/fansly-recorder) script.


## Updating

As this is a wip tool, new versions may be made available sporadically, There's a configuration option to "phone home" and check for updates automatically. However, the user will still need to use the built-in update command to update the binary, simply run:

```bash
./fansly-scraper update
```


## Program Status

<small>Maybe also a faq of sorts</small>

01/05/25 - Chat recording *may* miss some messages as it panics and reconnects to the websocket for  receiving chat messages and a message comes in while its reconnecting. I've mainly noticed this happens if a message hasn't been sent in a while. 

Macos users may need to accept/allow the use of notifications if you choose to enable system notifications as per [this issue](https://github.com/gen2brain/beeep/issues/67#issuecomment-2646474049) for the notification library used in the program.

For the time being:
- Currently the `Live Status` column in monitoring for the TUI only reflects the models you are monitoring. And currently doesn't update once the model goes live/offline. You can press `r` to reset/refresh the list to update it.
- It's recommended to leave m3u8_dl in the config set to false if you want things to download fast depending on your hardware. Do feel free to enable to give input/issues with it if you want.

### "Duplicate Files"

The scraper does attempt to scrape. Fansly API response for post can sometimes have the main media item (the preview image you see in the post) and another preview item (one that could be shown depending on timeline permissions), sometimes they can be the same image if that's how the model setup the post I guess.


## Disclaimer

> "Fansly" is operated by Select Media LLC.
>
> This repository and the provided content in it isn't in any way affiliated with, sponsored by, or endorsed by Select Media LLC or "Fansly".
>
> The developer of this script is not responsible for the end users' actions or any outcomes that may be taken upon the end users' account. Use at your own risk.

## Support the Project

If you find this tool useful and would like to support its development, you can donate using the following crypto addresses:

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
