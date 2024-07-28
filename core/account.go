package core

import (
    "fmt"
    "github.com/agnosto/fansly-scraper/auth"
    "github.com/agnosto/fansly-scraper/config"
)

type AccountInfo struct {
    Welcome string
    FollowedModels []auth.FollowedModel
}

func FetchAccountInfo(configPath string) (AccountInfo, error) {
    cfg, err := config.LoadConfig(configPath)
    if err != nil {
        return AccountInfo{}, fmt.Errorf("error loading config: %v", err)
    }

    accountInfo, err := auth.Login(cfg.Account.AuthToken, cfg.Account.UserAgent)
    if err != nil {
        return AccountInfo{}, fmt.Errorf("error logging in: %v", err)
    }

    welcome := fmt.Sprintf("Welcome %s | %s", accountInfo.DisplayName, accountInfo.Username)

    followedModels, err := auth.GetFollowedUsers(accountInfo.ID, cfg.Account.AuthToken, cfg.Account.UserAgent)
    if err != nil {
        return AccountInfo{}, fmt.Errorf("error getting followed models: %v", err)
    }

    return AccountInfo{
        Welcome: welcome,
        FollowedModels: followedModels,
    }, nil
}


//func EditConfig(configPath string) () {}
