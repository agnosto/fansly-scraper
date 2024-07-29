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

## ToDo:

- [ ] Test windows pathing for save location is handled correctly
- [ ] Fix editing config (broke in refactor lol)
- [ ] Fix m3u8 downloading for potential higher quality
- [ ] Fix sometimes saving video extension to images and vice versa
- [ ] Fix occasional <Access Denied> file download (rarely appears mostly from one model from short test)
- [ ] Properly handle going back during post fetching to cancel
- [ ] Properly handle going back to main menu once finished (downloading/interactions)
- [ ] More testing for timeline scraping (not subbed to any models atm)
- [ ] Implement message post (un)liking (ig if I really want it to be all-in-one)
- [ ] Handle getting messages with a selected model 
- [ ] Handle content downloading from messages 
- [ ] Handle getting model stories if available and saving
- [ ] Implement All selection to go through each download option
- [ ] Rethink about live monitoring (already have [this](https://github.com/agnosto/fansly-recorder) for recording lives, but might want to have this project use [services](https://github.com/kardianos/service) for managing lives? not really sure)
- [ ] I guess also save a version of models pfp & banner
- [ ] Add option to download purchases directly and save to model folder (default to creating purchases/{model id} if account is deleted)
- [ ] Add arguments to handle downloading model content (something like fansly-scraper -u {model username} -d [all|timeline|messages])
