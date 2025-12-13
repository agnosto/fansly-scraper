package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/agnosto/fansly-scraper/config"
	"github.com/agnosto/fansly-scraper/headers"
	"github.com/agnosto/fansly-scraper/interactions"
	"github.com/agnosto/fansly-scraper/logger"
	"github.com/agnosto/fansly-scraper/posts"
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
		configPath := config.GetConfigPath()
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			logger.Logger.Printf("[ERROR] Failed to load config %v", err)
			return likeUnlikeCompletedMsg{success: false, err: err}
		}

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()

			ctx := context.Background()

			// Create FanslyHeaders instance
			fanslyHeaders, err := headers.NewFanslyHeaders(cfg)
			if err != nil {
				logger.Logger.Printf("[ERROR] Failed to create headers: %v", err)
				return
			}

			// Fetch all timeline posts
			timelinePosts, err := posts.GetAllTimelinePosts(m.selectedModelId, "", fanslyHeaders, cfg.Options.PostLimit)
			if err != nil {
				logger.Logger.Printf("Error fetching timeline posts for %s: %v", m.selectedModel, err)
				return
			}

			// Process the posts
			if action == "like" {
				logger.Logger.Printf("[INFO] Starting To Like All Posts for %v", m.selectedModel)
				err = interactions.LikeAllPosts(ctx, timelinePosts, configPath)
			} else {
				logger.Logger.Printf("[INFO] Starting To Unlike All Posts for %v", m.selectedModel)
				err = interactions.UnlikeAllPosts(ctx, timelinePosts, configPath)
			}

			if err != nil {
				logger.Logger.Printf("Error %sing posts for %s: %v", action, m.selectedModel, err)
			}
		}()

		wg.Wait()
		logger.Logger.Printf("[DEBUG] Like/Unlike operation completed, sending likeUnlikeCompletedMsg")
		return likeUnlikeCompletedMsg{success: true, err: nil}
	}
}
