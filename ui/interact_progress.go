package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	//"os"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/interactions"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
	//"github.com/schollz/progressbar/v3"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type likeUnlikeCompletedMsg struct {
	success bool
	err     error
}

func (m *MainModel) HandleLikeUnlikeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case likeUnlikeCompletedMsg:
		logger.Logger.Printf("[DEBUG] Received likeUnlikeCompletedMsg: success=%v, err=%v", msg.success, msg.err)
		if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
		}
		logger.Logger.Printf("[DEBUG] Transitioning to MainMenuState")
		m.state = MainMenuState
		m.cursorPos = 0
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			m.state = FollowedModelsState
			m.cursorPos = 0
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			m.quit = true
			return m, tea.Quit
		}
	}
	logger.Logger.Printf("[DEBUG] HandleLikeUnlikeUpdate: Unhandled message type: %T", msg)
	return m, nil
}

func (m *MainModel) RenderLikeUnlikeMenu() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Liking/Unliking posts for %s\n\n", m.selectedModel))

	return sb.String()
}

func (m *MainModel) InitiateLikeUnlike(action string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadConfig(config.GetConfigPath())
		if err != nil {
			logger.Logger.Printf("[ERROR] Failed to load config %v", err)
		}

		//done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			//defer close(done)
			ctx := context.Background()

			// Fetch all timeline posts
			timelinePosts, err := posts.GetAllTimelinePosts(m.selectedModelId, cfg.Account.AuthToken, cfg.Account.UserAgent)
			if err != nil {
				logger.Logger.Printf("Error fetching timeline posts for %s: %v", m.selectedModel, err)
				//close(done)
				return
			}

			// Process the posts
			if action == "like" {
				logger.Logger.Printf("[INFO] Starting To Like All Posts for %v", m.selectedModel)
				err = interactions.LikeAllPosts(ctx, timelinePosts, cfg.Account.AuthToken, cfg.Account.UserAgent)
			} else {
				logger.Logger.Printf("[INFO] Starting To Unlike All Posts for %v", m.selectedModel)
				err = interactions.UnlikeAllPosts(ctx, timelinePosts, cfg.Account.AuthToken, cfg.Account.UserAgent)
			}

			if err != nil {
				logger.Logger.Printf("Error %sing posts for %s: %v", action, m.selectedModel, err)
			}
			//close(done)
		}()

		//return func() tea.Msg {
		//<-done
		wg.Wait()
		logger.Logger.Printf("[DEBUG] Like/Unlike operation completed, sending likeUnlikeCompletedMsg")
		//m.state = MainMenuState
		return likeUnlikeCompletedMsg{success: true, err: nil}
		//}
	}
}
