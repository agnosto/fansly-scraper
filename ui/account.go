package ui

import (
    //"github.com/agnosto/fansly-scraper/config"
    "github.com/agnosto/fansly-scraper/core"
)

func (m *MainModel) FetchAccInfo(configPath string) error {
    accountInfo, err := core.FetchAccountInfo(configPath)
    if err != nil {
        return err
    }

    m.welcome = accountInfo.Welcome
    m.followedModels = accountInfo.FollowedModels
    m.filteredModels = accountInfo.FollowedModels
    m.updateTable()
    m.state = FollowedModelsState

    return nil
}
