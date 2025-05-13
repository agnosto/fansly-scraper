package service

import (
	"encoding/json"
	"fmt"
	"github.com/agnosto/fansly-scraper/config"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ChatMessage represents a single chat message from the Fansly chat
type ChatMessage struct {
	MessageID     string    `json:"message_id"`
	Message       string    `json:"message"`
	MessageType   string    `json:"message_type"`
	Timestamp     int64     `json:"timestamp"`
	TimeInSeconds float64   `json:"time_in_seconds,omitempty"`
	TimeText      string    `json:"time_text,omitempty"`
	Author        Author    `json:"author"`
	RawData       string    `json:"raw_data,omitempty"`
	ReceivedAt    time.Time `json:"received_at"`
}

// Author represents the sender of a chat message
type Author struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	IsCreator bool     `json:"is_creator,omitempty"`
	IsStaff   bool     `json:"is_staff,omitempty"`
	TierInfo  TierInfo `json:"tier_info,omitempty"`
}

// TierInfo contains subscription tier information
type TierInfo struct {
	TierID    string `json:"tier_id,omitempty"`
	TierColor string `json:"tier_color,omitempty"`
	TierName  string `json:"tier_name,omitempty"`
}

// ChatRecorder handles recording chat messages from livestreams
type ChatRecorder struct {
	activeRecorders map[string]*chatRecordingSession
	mu              sync.Mutex
	logger          *log.Logger
	reconnectWait   time.Duration
	maxRetries      int
}

type chatRecordingSession struct {
	modelID      string
	username     string
	chatRoomID   string
	conn         *websocket.Conn
	messages     []ChatMessage
	outputFile   string
	startTime    time.Time
	stopChan     chan struct{}
	wg           sync.WaitGroup
	saveInterval time.Duration
	lastSaveTime time.Time
	mu           sync.Mutex
	isRunning    bool // Add a flag to track if the session is running
}

// NewChatRecorder creates a new chat recorder service
func NewChatRecorder(logger *log.Logger) *ChatRecorder {
	if logger == nil {
		logger = log.New(os.Stdout, "chat_recorder: ", log.LstdFlags)
	}
	cr := &ChatRecorder{
		activeRecorders: make(map[string]*chatRecordingSession),
		logger:          logger,
		reconnectWait:   5 * time.Second,
		maxRetries:      5,
	}
	return cr
}

// StartRecording begins recording chat for a livestream
func (cr *ChatRecorder) StartRecording(modelID, username, chatRoomID, outputFile string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Check if already recording for this model
	if _, exists := cr.activeRecorders[modelID]; exists {
		return fmt.Errorf("already recording chat for %s", username)
	}

	// Create the chat recording session
	session := &chatRecordingSession{
		modelID:      modelID,
		username:     username,
		chatRoomID:   chatRoomID,
		outputFile:   outputFile,
		startTime:    time.Now(),
		stopChan:     make(chan struct{}),
		saveInterval: 30 * time.Second, // Save messages every 30 seconds
		lastSaveTime: time.Now(),
		isRunning:    true,
	}

	// Ensure the directory exists
	dir := filepath.Dir(outputFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for chat log: %v", err)
	}

	// Start the recording goroutine
	cr.activeRecorders[modelID] = session
	session.wg.Add(1)
	go cr.recordChat(session)

	cr.logger.Printf("Started recording chat for %s to %s", username, outputFile)
	return nil
}

// StopRecording stops recording chat for a specific model
func (cr *ChatRecorder) StopRecording(modelID string) error {
	cr.mu.Lock()
	session, exists := cr.activeRecorders[modelID]
	if !exists {
		cr.mu.Unlock()
		return fmt.Errorf("no active chat recording for model ID %s", modelID)
	}

	// Mark the session as not running before closing the channel
	session.mu.Lock()
	session.isRunning = false
	session.mu.Unlock()

	delete(cr.activeRecorders, modelID)
	cr.mu.Unlock()

	// Signal the recording goroutine to stop
	close(session.stopChan)

	// Wait for the goroutine to finish with a timeout
	done := make(chan struct{})
	go func() {
		session.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// Normal completion
	case <-time.After(5 * time.Second):
		cr.logger.Printf("Warning: Timed out waiting for chat recording to stop for %s", session.username)
	}

	// Save any remaining messages
	if len(session.messages) > 0 {
		if err := cr.saveMessages(session); err != nil {
			cr.logger.Printf("Error saving final chat messages: %v", err)
		}
	}

	// Ensure the connection is closed
	if session.conn != nil {
		session.conn.Close()
		session.conn = nil
	}

	cr.logger.Printf("Stopped recording chat for %s", session.username)
	return nil
}

// StopAllRecordings stops all active chat recordings
func (cr *ChatRecorder) StopAllRecordings() {
	cr.mu.Lock()
	activeSessions := make([]*chatRecordingSession, 0, len(cr.activeRecorders))
	for _, session := range cr.activeRecorders {
		// Mark each session as not running
		session.mu.Lock()
		session.isRunning = false
		session.mu.Unlock()

		activeSessions = append(activeSessions, session)
	}
	cr.activeRecorders = make(map[string]*chatRecordingSession)
	cr.mu.Unlock()

	// Stop each session
	for _, session := range activeSessions {
		close(session.stopChan)

		// Wait with timeout
		done := make(chan struct{})
		go func(s *chatRecordingSession) {
			s.wg.Wait()
			close(done)
		}(session)

		// Wait with timeout
		select {
		case <-done:
			// Normal completion
		case <-time.After(5 * time.Second):
			cr.logger.Printf("Warning: Timed out waiting for chat recording to stop for %s", session.username)
		}

		// Save any remaining messages
		if len(session.messages) > 0 {
			if err := cr.saveMessages(session); err != nil {
				cr.logger.Printf("Error saving final chat messages: %v", err)
			}
		}

		// Ensure the connection is closed
		if session.conn != nil {
			session.conn.Close()
			session.conn = nil
		}
	}

	cr.logger.Printf("Stopped all chat recordings")
}

// recordChat handles the WebSocket connection and message recording
func (cr *ChatRecorder) recordChat(session *chatRecordingSession) {
	// Create a flag to track if we've already decremented the WaitGroup
	decrementedWG := false
	defer func() {
		// Only decrement WaitGroup if we haven't already
		if !decrementedWG {
			session.wg.Done()
			decrementedWG = true
		}
	}()

	defer func() {
		// Recover from any panics
		if r := recover(); r != nil {
			cr.logger.Printf("Recovered from panic in chat recorder: %v", r)

			// Close the connection if it exists
			if session.conn != nil {
				session.conn.Close()
				session.conn = nil
			}

			// Check if the session should still be running
			session.mu.Lock()
			isRunning := session.isRunning
			session.mu.Unlock()

			// If the session should still be running, restart the websocket connection
			if isRunning {
				cr.logger.Printf("Attempting to reconnect after panic for %s", session.username)
				time.Sleep(cr.reconnectWait) // Wait before reconnecting

				// Increment the WaitGroup before starting a new goroutine
				session.wg.Add(1)

				// Start a new goroutine
				go cr.recordChat(session)
			} else {
				cr.logger.Printf("Session for %s marked as not running, not reconnecting after panic", session.username)
			}
		} else {
			// This is a normal exit (not a panic)
			// Make sure to close the connection when exiting
			if session.conn != nil {
				session.conn.Close()
				session.conn = nil
			}
			// Mark the session as not running only if this was not a panic
			session.mu.Lock()
			session.isRunning = false
			session.mu.Unlock()
		}
	}()

	cr.logger.Printf("Starting chat recording for model %s (%s), chat room ID: %s, output file: %s",
		session.username, session.modelID, session.chatRoomID, session.outputFile)

	// Main connection loop - will retry until session is stopped
	for {
		// Check if the session is still running
		session.mu.Lock()
		isRunning := session.isRunning
		session.mu.Unlock()

		if !isRunning {
			cr.logger.Printf("Session for %s is no longer running, exiting connection loop", session.username)
			return
		}

		// Connect to the WebSocket
		if err := cr.connectAndAuthenticate(session); err != nil {
			cr.logger.Printf("Error connecting to chat: %v, will retry in %v", err, cr.reconnectWait)
			time.Sleep(cr.reconnectWait)
			continue
		}

		// Start message handling loop
		if err := cr.handleMessages(session); err != nil {
			cr.logger.Printf("Error in message handling: %v, will reconnect", err)
			// Close the connection before reconnecting
			if session.conn != nil {
				session.conn.Close()
				session.conn = nil
			}
			time.Sleep(cr.reconnectWait)
			continue
		}

		// If we get here, the message loop exited normally (session stopped)
		return
	}
}

// New helper function to handle connection and authentication
func (cr *ChatRecorder) connectAndAuthenticate(session *chatRecordingSession) error {
	// Load config
	cfg, err := config.LoadConfig(config.GetConfigPath())
	if err != nil {
		return fmt.Errorf("error loading config: %v", err)
	}

	// Connect to the WebSocket server
	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
	}
	cr.logger.Printf("Connecting to chat WebSocket for %s", session.username)
	conn, resp, err := dialer.Dial("wss://chatws.fansly.com/?v=3", http.Header{
		"Origin":     []string{"https://fansly.com"},
		"User-Agent": []string{cfg.Account.UserAgent},
	})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("error connecting to chat WebSocket: %v (status: %s)", err, resp.Status)
		}
		return fmt.Errorf("error connecting to chat WebSocket: %v", err)
	}
	cr.logger.Printf("Successfully connected to chat WebSocket for %s", session.username)
	session.conn = conn

	// Set up error handling for the connection
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})
	conn.SetCloseHandler(func(code int, text string) error {
		cr.logger.Printf("WebSocket connection closed: %d %s", code, text)
		return nil
	})

	// Send authentication message
	authMsg := struct {
		Type int    `json:"t"`
		Data string `json:"d"`
	}{
		Type: 1,
		Data: fmt.Sprintf(`{"token":"%s","v":3}`, cfg.Account.AuthToken),
	}
	cr.logger.Printf("Sending authentication message to chat server")
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("error sending auth message: %v", err)
	}

	// Wait for auth response with timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var authResp struct {
		Type int             `json:"t"`
		Data json.RawMessage `json:"d"`
	}
	err = conn.ReadJSON(&authResp)
	conn.SetReadDeadline(time.Time{}) // Reset deadline
	if err != nil {
		conn.Close()
		return fmt.Errorf("error receiving auth response: %v", err)
	}
	cr.logger.Printf("Received auth response: type=%d", authResp.Type)

	// Check if auth was successful
	if authResp.Type != 1 && authResp.Type != 2 {
		conn.Close()
		return fmt.Errorf("authentication failed: unexpected response type %d", authResp.Type)
	}

	// Join the chat room
	joinMsg := struct {
		Type int    `json:"t"`
		Data string `json:"d"`
	}{
		Type: 46001,
		Data: fmt.Sprintf(`{"chatRoomId":"%s"}`, session.chatRoomID),
	}
	cr.logger.Printf("Joining chat room: %s", session.chatRoomID)
	if err := conn.WriteJSON(joinMsg); err != nil {
		conn.Close()
		return fmt.Errorf("error joining chat room: %v", err)
	}

	return nil
}

// New helper function to handle message processing
func (cr *ChatRecorder) handleMessages(session *chatRecordingSession) error {
	// Set up ticker for periodic pings
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Set up ticker for periodic saves
	saveTicker := time.NewTicker(session.saveInterval)
	defer saveTicker.Stop()

	// Set a reasonable read deadline for the first message
	session.conn.SetReadDeadline(time.Now().Add(45 * time.Second))

	for {
		// Check if the session is still running
		session.mu.Lock()
		isRunning := session.isRunning
		session.mu.Unlock()

		if !isRunning {
			return nil // Exit normally if session is stopped
		}

		select {
		case <-session.stopChan:
			cr.logger.Printf("Received stop signal for %s", session.username)
			return nil

		case <-pingTicker.C:
			// Send ping message
			pingMsg := struct {
				Type int    `json:"t"`
				Data string `json:"d"`
			}{
				Type: 0,
				Data: "p",
			}
			cr.logger.Printf("Sending ping to keep connection alive")
			if err := session.conn.WriteJSON(pingMsg); err != nil {
				return fmt.Errorf("error sending ping: %v", err)
			}

		case <-saveTicker.C:
			// Save messages periodically
			if len(session.messages) > 0 {
				cr.logger.Printf("Periodic save triggered for %s (%d messages)",
					session.username, len(session.messages))
				if err := cr.saveMessages(session); err != nil {
					cr.logger.Printf("Error saving chat messages: %v", err)
				}
			}

		default:
			// Read the next message with a reasonable timeout
			session.conn.SetReadDeadline(time.Now().Add(45 * time.Second))
			messageType, message, err := session.conn.ReadMessage()
			session.conn.SetReadDeadline(time.Time{}) // Reset the deadline after read

			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return fmt.Errorf("websocket closed normally")
				}

				// Check if it's a timeout (which is expected with our approach)
				if strings.Contains(err.Error(), "timeout") {
					// This is just a timeout from our deadline, not a real error
					continue
				}

				return fmt.Errorf("error reading message: %v", err)
			}

			// Only process text messages
			if messageType != websocket.TextMessage {
				cr.logger.Printf("Received non-text message type: %d", messageType)
				continue
			}

			// Parse the message
			var msg struct {
				Type int             `json:"t"`
				Data json.RawMessage `json:"d"`
			}
			if err := json.Unmarshal(message, &msg); err != nil {
				cr.logger.Printf("Error unmarshaling message: %v", err)
				continue
			}

			// Process the message
			if msg.Type == 10000 {
				// This is a chat message event
				chatMsg, err := cr.parseMessage(string(msg.Data), session.startTime)
				if err != nil {
					cr.logger.Printf("Error parsing message: %v", err)
					continue
				}
				if chatMsg != nil {
					cr.logger.Printf("Received chat message from %s: %s",
						chatMsg.Author.Name, chatMsg.Message)
					session.mu.Lock()
					session.messages = append(session.messages, *chatMsg)
					messageCount := len(session.messages)
					session.mu.Unlock()

					// If we have accumulated a lot of messages, save them
					if messageCount >= 100 {
						cr.logger.Printf("Saving %d accumulated messages", messageCount)
						if err := cr.saveMessages(session); err != nil {
							cr.logger.Printf("Error saving chat messages: %v", err)
						}
					}
				}
			}
		}
	}
}

// parseMessage converts a raw WebSocket message to a ChatMessage
func (cr *ChatRecorder) parseMessage(data string, startTime time.Time) (*ChatMessage, error) {
	//cr.logger.Printf("Parsing message data: %s", data)

	// Try to unmarshal the data as a JSON object
	var rawData struct {
		ServiceID int    `json:"serviceId"`
		Event     string `json:"event"`
	}

	// First, check if the data is a JSON string that needs to be unescaped
	if data[0] == '"' && data[len(data)-1] == '"' {
		// This is a JSON string that needs to be unescaped
		var unquotedData string
		if err := json.Unmarshal([]byte(data), &unquotedData); err != nil {
			return nil, fmt.Errorf("error unquoting JSON string: %v", err)
		}
		data = unquotedData
	}

	if err := json.Unmarshal([]byte(data), &rawData); err != nil {
		// Try to handle other message formats
		var msg struct {
			Type int             `json:"t"`
			Data json.RawMessage `json:"d"`
		}
		if err := json.Unmarshal([]byte(data), &msg); err != nil {
			return nil, fmt.Errorf("error unmarshaling message data: %v", err)
		}

		// Only process chat messages (type 10000)
		if msg.Type != 10000 {
			return nil, nil
		}

		// Parse the message data
		var chatData struct {
			MessageID  string `json:"id"`
			Content    string `json:"content"`
			SenderID   string `json:"senderId"`
			SenderName string `json:"senderName"`
			CreatedAt  int64  `json:"createdAt"`
			IsCreator  bool   `json:"isCreator"`
			IsStaff    bool   `json:"isStaff"`
			TierID     string `json:"tierId"`
			TierColor  string `json:"tierColor"`
			TierName   string `json:"tierName"`
		}
		if err := json.Unmarshal(msg.Data, &chatData); err != nil {
			return nil, fmt.Errorf("error unmarshaling chat data: %v", err)
		}

		// Calculate time in seconds since stream start
		receivedAt := time.Now()
		elapsedSeconds := receivedAt.Sub(startTime).Seconds()

		// Format time as MM:SS
		minutes := int(elapsedSeconds) / 60
		seconds := int(elapsedSeconds) % 60
		timeText := fmt.Sprintf("%02d:%02d", minutes, seconds)

		cr.logger.Printf("Successfully parsed chat message from %s", chatData.SenderName)
		return &ChatMessage{
			MessageID:     chatData.MessageID,
			Message:       chatData.Content,
			MessageType:   "text_message",
			Timestamp:     chatData.CreatedAt,
			TimeInSeconds: elapsedSeconds,
			TimeText:      timeText,
			Author: Author{
				ID:        chatData.SenderID,
				Name:      chatData.SenderName,
				IsCreator: chatData.IsCreator,
				IsStaff:   chatData.IsStaff,
				TierInfo: TierInfo{
					TierID:    chatData.TierID,
					TierColor: chatData.TierColor,
					TierName:  chatData.TierName,
				},
			},
			RawData:    string(data),
			ReceivedAt: receivedAt,
		}, nil
	}

	// Only process chat room messages (service ID 46)
	if rawData.ServiceID != 46 {
		cr.logger.Printf("Ignoring message with service ID: %d", rawData.ServiceID)
		return nil, nil
	}

	// Now parse the event JSON which is a string
	var eventData struct {
		Type            int `json:"type"`
		ChatRoomMessage struct {
			ID                string `json:"id"`
			ChatRoomID        string `json:"chatRoomId"`
			SenderID          string `json:"senderId"`
			Content           string `json:"content"`
			Type              int    `json:"type"`
			Private           int    `json:"private"`
			Metadata          string `json:"metadata"`
			CreatedAt         int64  `json:"createdAt"`
			Username          string `json:"username"`
			DisplayName       string `json:"displayname"`
			UsernameColor     string `json:"usernameColor"`
			AccountFlags      int    `json:"accountFlags"`
			Attachments       []any  `json:"attachments"`
			Embeds            []any  `json:"embeds"`
			ChatRoomAccountID string `json:"chatRoomAccountId"`
		} `json:"chatRoomMessage"`
	}

	if err := json.Unmarshal([]byte(rawData.Event), &eventData); err != nil {
		return nil, fmt.Errorf("error unmarshaling event data: %v", err)
	}

	// Only process text messages (type 10)
	if eventData.Type != 10 {
		cr.logger.Printf("Ignoring event with type: %d", eventData.Type)
		return nil, nil
	}

	// Parse the metadata for additional user info
	var metadata struct {
		SenderIsCreator    bool `json:"senderIsCreator"`
		SenderIsStaff      bool `json:"senderIsStaff"`
		SenderIsFollowing  bool `json:"senderIsFollowing"`
		SenderSubscription struct {
			TierID    string `json:"tierId"`
			TierColor string `json:"tierColor"`
			TierName  string `json:"tierName"`
		} `json:"senderSubscription"`
	}

	// Handle potentially missing metadata
	if eventData.ChatRoomMessage.Metadata != "" {
		if err := json.Unmarshal([]byte(eventData.ChatRoomMessage.Metadata), &metadata); err != nil {
			cr.logger.Printf("Warning: could not parse message metadata: %v", err)
		}
	}

	// Calculate time in seconds since stream start
	receivedAt := time.Now()
	elapsedSeconds := receivedAt.Sub(startTime).Seconds()

	// Format time as MM:SS
	minutes := int(elapsedSeconds) / 60
	seconds := int(elapsedSeconds) % 60
	timeText := fmt.Sprintf("%02d:%02d", minutes, seconds)

	cr.logger.Printf("Successfully parsed chat message from %s", eventData.ChatRoomMessage.DisplayName)
	return &ChatMessage{
		MessageID:     eventData.ChatRoomMessage.ID,
		Message:       eventData.ChatRoomMessage.Content,
		MessageType:   "text_message",
		Timestamp:     eventData.ChatRoomMessage.CreatedAt,
		TimeInSeconds: elapsedSeconds,
		TimeText:      timeText,
		Author: Author{
			ID:        eventData.ChatRoomMessage.SenderID,
			Name:      eventData.ChatRoomMessage.DisplayName,
			IsCreator: metadata.SenderIsCreator,
			IsStaff:   metadata.SenderIsStaff,
			TierInfo: TierInfo{
				TierID:    metadata.SenderSubscription.TierID,
				TierColor: metadata.SenderSubscription.TierColor,
				TierName:  metadata.SenderSubscription.TierName,
			},
		},
		RawData:    data,
		ReceivedAt: receivedAt,
	}, nil
}

// saveMessages saves the current messages to the output file
func (cr *ChatRecorder) saveMessages(session *chatRecordingSession) error {
	session.mu.Lock()
	messages := session.messages
	messageCount := len(messages)
	session.messages = []ChatMessage{} // Clear the messages
	session.lastSaveTime = time.Now()
	session.mu.Unlock()

	cr.logger.Printf("Attempting to save %d chat messages for %s", messageCount, session.username)

	// Ensure the directory exists
	dir := filepath.Dir(session.outputFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory for chat file: %v", err)
	}

	// Check if the file exists
	var existingMessages []ChatMessage
	if _, err := os.Stat(session.outputFile); err == nil {
		// File exists, read existing messages
		cr.logger.Printf("Reading existing chat file: %s", session.outputFile)
		data, err := os.ReadFile(session.outputFile)
		if err != nil {
			return fmt.Errorf("error reading existing chat file: %v", err)
		}

		if err := json.Unmarshal(data, &existingMessages); err != nil {
			return fmt.Errorf("error parsing existing chat file: %v", err)
		}

		cr.logger.Printf("Found %d existing messages in chat file", len(existingMessages))
	} else {
		cr.logger.Printf("Creating new chat file: %s", session.outputFile)
		// Create an empty file if it doesn't exist and we have no messages
		if messageCount == 0 {
			if err := os.WriteFile(session.outputFile, []byte("[]"), 0644); err != nil {
				return fmt.Errorf("error creating empty chat file: %v", err)
			}
			cr.logger.Printf("Created empty chat file: %s", session.outputFile)
			return nil
		}
	}

	// Combine existing and new messages
	allMessages := append(existingMessages, messages...)
	cr.logger.Printf("Total messages to save: %d", len(allMessages))

	// Sort messages by timestamp
	sort.Slice(allMessages, func(i, j int) bool {
		return allMessages[i].Timestamp < allMessages[j].Timestamp
	})

	// Write the combined messages to the file
	data, err := json.MarshalIndent(allMessages, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling chat messages: %v", err)
	}

	if err := os.WriteFile(session.outputFile, data, 0644); err != nil {
		return fmt.Errorf("error writing chat messages to file: %v", err)
	}

	cr.logger.Printf("Successfully saved %d chat messages for %s", len(allMessages), session.username)
	return nil
}

// IsRecording checks if chat recording is active for a model
func (cr *ChatRecorder) IsRecording(modelID string) bool {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	_, exists := cr.activeRecorders[modelID]
	return exists
}

// GetChatFilename generates a chat log filename based on the VOD filename
func (cr *ChatRecorder) GetChatFilename(vodFilename string) string {
	ext := filepath.Ext(vodFilename)
	baseFilename := strings.TrimSuffix(vodFilename, ext)
	return baseFilename + "_chat.json"
}
