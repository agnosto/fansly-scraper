# Fansly Scraper 

### ⚠ This is currently under development, everything is still being planned out and tested. Feel free to create issues/pr's to assist in the creation of this project.

### [Tracker](./TRACKER.md) will hold a list of things to do for the project, as well as act as a kind of roadmap.

### I also recommend reading [Program Status](https://github.com/agnosto/fansly-scraper?tab=readme-ov-file#program-status) for more info.


## A simple all in one fansly interaction tool.

The program will automatically move/download an example config into the config path from either the current directory or the github repo respectively, you will need to edit the config to run the program.

[ffmpeg](https://ffmpeg.org/) is not needed, but for potentially saving higher quality videos it is required.

## Installing the program

### Releases

Pre-compiled binaries can be downloaded from the [releases](https://github.com/agnosto/fansly-scraper/releases) section.


### Manual Compile

```bash
git clone https://github.com/agnosto/fansly-scraper && cd fansly-scraper 

go build -v -o fansly-scraper 

# run the binary
./fansly-scraper
```

### Install Via Go

Need to change project directory structure before it can be used

```bash
go install github.com/agnosto/fansly-scraper
```


## Updating

As this is a wip tool, new versions may be made available sporadically, I've avoided having the program "phone home" and check for updates automatically. However, there is a built-in update arguement that will check for new releases and update the binary, simply run:

```bash
./fansly-scraper update
```

## Get fansly account token

### Method 1:
1. Go to [fansly](https://fansly.com) and login and open devtools (ctrl+shift+i / F12)
2. In network request, type `method:GET api` and click one of the requests
3. Look under `Request Headers` and look for `Authorization` and copy the value

### Method 2 (Recommended) :
1. Go to [fansly](https://fansly.com) and login and open devtools (ctrl+shift+i / F12)
2. Click on `Storage` and then `Local Storage`
3. Look for `session_active_session` and copy the `token` value

(images at a later date)

## Program Status

<small>Maybe also a faq of sorts</small>

For the time being:
- It's recommended to leave m3u8_dl in the config set to false.
- Press `ESC` once done downloading/(un)liking to be able to go back.

### "Duplicate Files"

The scraper does what it says, it scrapes. Fansly API response for post can sometimes have the main media item (the preview image you see in the post) and another preview item (one that could be shown depending on timeline permissions), sometimes they can be the same image if that's how the model setup the post I guess.


## Disclaimer

> "Fansly" is operated by Select Media LLC.
>
> This repository and the provided content in it isn't in any way affiliated with, sponsored by, or endorsed by Select Media LLC or "Fansly".
>
> The developer of this script is not responsible for the end users' actions or any outcomes that may be taken upon the end users' account. Use at your own risk.
