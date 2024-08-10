# Scraper Tracker

## Currently Working:

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


## ToDo:

- [ ] Fix editing config option (broke in refactor lol)
- [ ] Fix m3u8 downloading for potential higher quality
- [ ] Fix sometimes saving video extension to images and vice versa
- [ ] Fix occasional <Access Denied> file download (rarely appears mostly from one model from short test, most likely rate limit issue)
- [ ] Properly handle going back during post fetching to cancel
- [ ] Properly handle going back to main menu once finished (downloading/interactions)
- [ ] More testing for timeline scraping (not subbed to any models atm)
- [ ] Implement message post (un)liking (ig if I really want it to be all-in-one)
- [ ] Rethink about live monitoring (already have [this](https://github.com/agnosto/fansly-recorder) for recording lives, but might want to have this project use [services](https://github.com/kardianos/service) for managing lives? not really sure)
- [ ] I guess also save a version of models pfp & banner
- [ ] Add option to download purchases directly and save to model folder (default to creating purchases/{model id} if account is deleted)
- [ ] Add simple web dashboard to view images and watch videos/lives (was contemplating on adding but maybe down the line)
- [ ] Fix livestream recording as a service to save/convert in background (i might just make it a go version of my python script but allow multiple models eg ./fansly-scraper -m user1 user2 user3 user4 instead of a service)
