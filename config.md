# Configuration Guide

> [!WARNING]
> If the config file is fully setup prior to the security headers getting populated, it may reset the Live Settings to defaults/empty. It is recommended for the time being to set your auth_token and user_agent first and run the script to populate those fields and then continue on. You can view an example config file [here](./example-config.toml)

This document outlines available options for the Scraper.

## Account Settings
| Setting | Description | Required | Example |
|---------|-------------|----------|---------|
| auth_token | Your Fansly authentication token | Yes | "xxxxxx" |
| user_agent | Browser user agent string | Yes | "Mozilla/5.0..." |

<details>
<summary><strong>Getting your token</strong></summary>

### Method 1 (Recommended) special thanks to [prof79](https://github.com/prof79/)'s wiki for this:
1. Go to [fansly](https://fansly.com) and login and open devtools (ctrl+shift+i / F12)
2. In devtools, go to the Console Tab and Paste the following: 
```javascript
console.clear(); // cleanup console
const activeSession = localStorage.getItem("session_active_session"); // get required key
const { token } = JSON.parse(activeSession); // parse the json data
console.log('%c➡️ Authorization_Token =', 'font-size: 12px; color: limegreen; font-weight: bold;', token); // show token
console.log('%c➡️ User_Agent =', 'font-size: 12px; color: yellow; font-weight: bold;', navigator.userAgent); // show user-agent
```

### Method 2:
1. Go to [fansly](https://fansly.com) and login and open devtools (ctrl+shift+i / F12)
2. Click on `Storage` and then `Local Storage`
3. Look for `session_active_session` and copy the `token` value
</details>

## Options
| Setting | Description | Default | Example |
|---------|-------------|---------|---------|
| save_location | Base directory for downloads, on windows repalce backslashes ("\\") in the path with forward slashes ("/") | Required | "/home/user/content" |
| m3u8_dl | Use m3u8 downloader for saving content | false | true/false |
| check_updates | Check for new updates on launch |false | true/false |


## Live Settings
| Setting | Description | Default | Example |
|---------|-------------|---------|---------|
| save_location | Custom path for livestreams | Empty (uses save_location from Options) | "/home/user/streams" |
| vods_file_extension | File extension for recordings | ".ts" | ".ts" or ".mp4" |
| ffmpeg_convert | Convert to MP4 after recording | true | true/false |
| generate_contact_sheet | Create preview thumbnails | true | true/false |
| filename_template | Template for file naming | See below | "{model_username}_{date}" |
| date_format | Date format in filenames | "20060102_150405" | "2006-01-02_15:04:05" |

### Video File Extensions
Common options for `vods_file_extension`:
- `.ts` - Transport Stream (recommended for live recordings)
- `.mp4` - Most widely supported video format
- `.mkv` - Matroska format, supports multiple audio/subtitle tracks
- `.mov` - QuickTime Movie format
- `.avi` - Audio Video Interleave format
- `.webm` - WebM format, good for web compatibility

Note: `.ts` is recommended for live recordings as it handles interruptions better and can be played directly in VLC/MPV players.

### Filename Template Variables
- `{model_username}`: Creator's username
- `{date}`: Recording date/time
- `{streamId}`: Unique stream identifier
- `{streamVersion}`: Stream version (automatically prefixed with 'v')

### Date Format Options
The date format uses Go's time formatting syntax:
- `2006`: Year
- `01`: Month
- `02`: Day
- `15`: Hour (24h)
- `04`: Minute
- `05`: Second

Common formats:
- `20060102_150405`: 20240215_134530
- `2006-01-02_15:04:05`: 2024-02-15_13:45:30
- `2006-01-02`: 2024-02-15
- `15:04:05`: 13:45:30
- `Jan 02 2006`: Feb 15 2024
- `January 02 2006`: February 15 2024
- `Mon Jan 02 2006`: Thu Feb 15 2024
- `02-01-2006`: 15-02-2024

## Security Headers
These are automatically managed by the application:
- `device_id`: Unique device identifier
- `session_id`: Current session ID
- `check_key`: Security check key
- `last_updated`: Last refresh timestamp
