# Scraper Tracker

## Currently Working/Done:

- [x] Tui dispaly
- [x] Tui navigation
- [x] Config Generation and Parsing
- [x] Get User Information
- [x] Get Following List
- [x] Filtering Follow List
- [x] Ensure proper timeline access permissions (want to test more)
- [x] Download Model Timeline (partial)
- [x] Timeline post (un)liking (probably want to add the appropriate fansly-* headers)
- [x] Update command argument to get new binary releases
- [x] Handle getting messages with a selected model 
- [x] Handle content downloading from messages
- [x] Add arguments to handle downloading model content (something like fansly-scraper -u {model username} -d [all|timeline|messages|stories])
- [x] Handle getting model stories if available and saving
- [x] Implement All selection to go through each download option
- [x] Test windows pathing for save location is handled correctly
- [x] Fix livestream recording as a service to save/convert in background (I just made it a go version of my python script which uses models you've enabled to monitor)
- [x] Fix monitoring service starting on app launch 
- [x] Fix models being output twice in monitoring 
- [x] Fix sometimes saving video extension to images and vice versa(haven't seen happen in a while)
- [x] Better CLI help output
- [x] Fix m3u8 downloading for potential higher quality (want to do more testing on different hardware, but should be fine for the most part)
- [x] Fix editing config option (broke in refactor lol)
- [x] Add option to download purchases directly and save to model folder (default to creating {model id}/purchases if account is deleted)
- [x] Handle pid removal on windows when using cli monitoring 
- [x] Configurable filename for lives and separate save path
- [x] Properly handle going back to main menu once finished (downloading/interactions)
- [x] Fix progress time during file download
- [x] Add new menu post download/interact, asking to continue with another creator, quit, etc.
- [x] Properly handle monitoring in TUI
- [x] Notifications for monitored creators livestreams
- [x] Handle displaying all followed creators (in cases of people who follow {original limit}+)
- [x] Standardize DB handling
- [x] Handle hashing and db storing of livestreams, handling post processing cases

## ToDo:

- [ ] Add option to limit resolution for downloading (post and livestreams)
- [ ] Archive metadata for downloaded post
- [ ] Properly handle live status in monitoring TUI 
    - [ ] Update to offline once offline
    - [ ] Update status regardless of monitoring status
- [ ] Fix progress bar rendering error (eg. ⠋ Downloading xxxxx_xxxxx.png (126 kB) (126 kB, 2.6 MB/s, 2566525 it/s) [0s] 13s])
- [ ] Fix occasional `<Access Denied>` file download (most likely rate limit issue)
- [ ] Properly handle going back during post fetching to cancel
- [ ] More testing for timeline scraping (not subbed to any models atm)
- [ ] I guess also save a version of models pfp & banner

## Limbo:

- [ ] Rethink about live monitoring (already have [this](https://github.com/agnosto/fansly-recorder) for recording lives, but might want to have this project use [services](https://github.com/kardianos/service) for managing lives? not really sure)
- [ ] Implement message post (un)liking (ig if I really want it to be all-in-one)
- [ ] Add simple web dashboard to view images and watch videos/lives (was contemplating on adding but maybe down the line)
