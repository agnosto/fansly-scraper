# Configuration Guide

> [!WARNING]
> When updating the application, some settings might be reset to their default values if they were not present in the new version's default configuration. Notably, `skip_previews` might reset to `true`. Please review your settings, especially in the `[options]` section, after an update. You can view an example config file [here](./example-config.toml)

## Location

Where the config file is located

### Windows:
`%APPDATA%\fansly-scraper\config.toml`

### Linux and Macos:
`~/.config/fansly-scraper/config.toml`

Note: If a `config.toml` file exists in the same directory as the executable, that file will be used instead.

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
| save_location | Base directory for downloads, on windows replace backslashes ("\\") in the path with forward slashes ("/") | Required | "/home/user/content" |
| m3u8_dl | Use m3u8 downloader for saving content | false | true/false |
| check_updates | Check for new updates on launch | false | true/false |
| [skip_previews](#preview-files) | Skip downloading preview images/videos for posts and messages | true | true/false |
| [use_content_as_filename](#readable-filenames) | Generate human-readable filenames from post/message content. | false | true/false |
| [content_filename_template](#readable-filenames) | Template for readable filenames. See variables below. | "{date}-{content}_{index}" | "{date}-{content}" |
| [download_media_type](#filtering-by-media-type)   | Download only specific media. Options: `all`, `images`, `videos`, `audio.` | "all"   | "videos"  |
| [skip_downloaded_posts](#skipping-processed-posts) | Skip posts that have already been processed to speed up subsequent runs.   | false  | true/false  |

### Filtering by Media Type

The `download_media_type` option allows you to save bandwidth and storage by only downloading the content you want. This is useful if you are only interested in a creator's images or videos, for example.

- **`all`**: Downloads all media types (images, videos, and audio). This is the default.
- **`images`** or **`image`**: Downloads only images.
- **`videos`** or **`video`**: Downloads only videos.
- **`audios`** or **`audio`**: Downloads only audio files.

The setting is not case-sensitive, and both singular and plural forms work (e.g., `videos` and `video` are treated the same). This filter applies to all content types, including Timeline, Messages, Stories, and Purchases.

### Skipping Processed Posts

Set `skip_downloaded_posts` to `true` to dramatically speed up re-running the scraper on a creator you have already downloaded. When enabled, the application keeps a record of every post it successfully processes in its local database (`downloads.db`). On future runs, it will skip any post ID that is already in its records, avoiding the need to re-fetch and check media for that post.

> [!WARNING]
> Due to API rate limits or network issues, it's possible for the application to mark a post as "processed" even if some of its media failed to download. If you suspect content is missing from a previous run, it is recommended to temporarily set `skip_downloaded_posts = false` to force the scraper to re-check all of the creator's posts for any missing files.

### Readable Filenames

When use_content_as_filename is set to true, the scraper will generate filenames based on the content of a post or message, making your archive much easier to browse.

- For Timeline Posts & Messages with Text: The filename is generated using the content_filename_template.
- For Purchases, Stories, or content without text: A fallback template ({date}_{model_name}_{mediaId}_{index}) is used automatically to ensure files are still well-organized.
- If use_content_as_filename is false: The original ID-based naming scheme ({postId}_{mediaId}) is used.

#### Available Template Variables:

- {date}: The date the content was posted (YYYYMMDD format).
- {content}: The first 45 characters of the post/message text, sanitized for filesystem safety.
- {index}: A number (0, 1, 2...) for each piece of media in a single post/message.
- {postId}: The unique ID of the post or message.
- {mediaId}: The unique ID of the media file.
- {model_name}: The username of the content creator.


### Preview Files

Preview files are typically lower quality or shorter versions of the main media content. They are often used as thumbnails or quick previews on the platform. When `skip_previews` is set to `true`, only the main media files will be downloaded, which can:

- Save storage space
- Reduce download time
- Avoid duplicate/similar content

Preview files are usually named with a `_preview` suffix (e.g., `postid_mediaid_preview.jpg`).

## Live Settings

| Setting | Description | Default | Example |
|---------|-------------|---------|---------|
| save_location | Custom path for livestreams | Empty (uses save_location from Options) | "/home/user/streams" |
| [vods_file_extension](#video-file-extensions) | File extension for recordings | ".ts" | ".ts" or ".mp4" |
| ffmpeg_convert | Convert to MP4 after recording | true | true/false |
| generate_contact_sheet | Create preview thumbnails | true | true/false |
| use_mt_for_contact_sheet | Use [mt](https://github.com/mutschler/mt) for better thumbnails if its installed | false | true/false |
| [filename_template](#filename-template-variables) | Template for file naming | See below | "{model_username}_{date}" |
| [date_format](#date-format-options) | Date format in filenames | "20060102_150405" | "2006-01-02_15:04:05" |
| [record_chat](#recorded-chat) | Save chat messages from streams to a json file* | true | true/false |
| [ffmpeg_recording_options](#custom-ffmpeg-options) | Custom FFmpeg arguments for the initial stream recording. Leave empty to use application defaults. | "" | "-c:v libx264 -crf 23" |
| [ffmpeg_conversion_options](#custom-ffmpeg-options)  | Custom FFmpeg arguments for the post-recording conversion. Overrides the default -c copy. | "" | "-c copy -movflags +faststart" |

### Custom FFmpeg Options
For advanced users, you can directly control the arguments passed to FFmpeg for both recording and post-processing.

>[!WARNING]
>Incorrect FFmpeg syntax can cause your recordings to fail. It is highly recommended to test your command-line arguments directly with FFmpeg before using them in the configuration file.

- ffmpeg_recording_options: This replaces the default set of arguments used to record the live stream. If left empty, the application uses its built-in defaults which are optimized for stable stream capture and reconnection (-c copy -movflags use_metadata_tags -reconnect 300 ...). This option allows you to re-encode on the fly, map specific streams, or apply filters during the recording process.
- ffmpeg_conversion_options: This replaces the default argument (-c copy) used during the post-processing step when ffmpeg_convert is set to true. This is useful for tasks like adding faststart flags for web playback, changing audio codecs, or embedding metadata without re-encoding the entire video.

### Recorded chat

Chat messages are saved in a json format to be compatible with the player from this amazing archive project: https://archive.ragtag.moe/player

It allows you to play local videos with the chat to have full context of streams with timestamps of the messages (as close as possible). Any empty messages are most likely tips that had no messages associated with it, I may also save those as the amount tipped.

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

## Notifications

| Setting | Description | Default | Example |
|---------|-------------|---------|---------|
| enabled | Enable notifications | false | true/false |
| system_notify | Show system notifications | true | true/false |
| discord_webhook | Discord webhook URL | "" | "https://discord.com/api/webhooks/..." |
| discord_mention_id | Discord user/role ID to mention | "" | "123456789012345678" |
| telegram_bot_token | Telegram bot token | "" | "1234567890:ABCDEF..." |
| telegram_chat_id | Telegram chat ID | "" | "123456789" |
| notify_on_live_start | Send notification when stream starts | true | true/false |
| notify_on_live_end | Send notification when stream ends | false | true/false |

### Discord Notifications

To set up Discord notifications:
1. Create a webhook in your Discord server (Server Settings → Integrations → Webhooks)
2. Copy the webhook URL to `discord_webhook`
3. For the `discord_mention_id` field:
   - For user mentions: Simply add the user ID (e.g., "123456789012345678")
   - For role mentions: Prefix the role ID with "role:" (e.g., "role:123456789012345678")

To get a user or role ID:
1. Enable Developer Mode in Discord (User Settings → App Settings → Advanced → Developer Mode)
2. Right-click on a user or role and select "Copy ID"

### Telegram Notifications

To set up Telegram notifications:
1. Create a bot using [BotFather](https://t.me/botfather) and get the token
2. Add the bot to your chat or group
3. Get your chat ID (you can use [userinfobot](https://t.me/userinfobot) or send a message to your bot and check `https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getUpdates`)
4. Add the bot token to `telegram_bot_token` and chat ID to `telegram_chat_id`

## Security Headers

These are automatically managed by the application:
- `device_id`: Unique device identifier
- `session_id`: Current session ID
- `check_key`: Security check key
- `last_updated`: Last refresh timestamp
