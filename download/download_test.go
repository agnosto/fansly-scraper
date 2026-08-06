package download

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agnosto/fansly-scraper/posts"
)

func TestParseUnixTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected time.Time
	}{
		{
			name:     "Zero",
			input:    0,
			expected: time.Time{},
		},
		{
			name:     "Negative",
			input:    -100,
			expected: time.Time{},
		},
		{
			name:     "Unix Seconds",
			input:    1785342602,
			expected: time.Unix(1785342602, 0),
		},
		{
			name:     "Unix Milliseconds",
			input:    1785342602000,
			expected: time.Unix(1785342602, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseUnixTimestamp(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("parseUnixTimestamp(%d) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractTimestamp(t *testing.T) {
	postTime := time.Unix(1785342602, 0)
	mediaTime := time.Unix(1785342552, 0)

	// 1. Timestamp from Post
	p := posts.Post{ID: "123", CreatedAt: 1785342602}
	am := posts.AccountMedia{ID: "456", CreatedAt: 1785342552}
	item := posts.MediaItem{ID: "789", CreatedAt: 1785342500}

	ts := extractTimestamp(p, am, item)
	if !ts.Equal(postTime) {
		t.Errorf("expected timestamp from post %v, got %v", postTime, ts)
	}

	// 2. Timestamp from Message
	msg := posts.Message{ID: "123", CreatedAt: 1785342602}
	ts = extractTimestamp(msg, am, item)
	if !ts.Equal(postTime) {
		t.Errorf("expected timestamp from message %v, got %v", postTime, ts)
	}

	// 3. Timestamp from Story
	story := posts.Story{ID: "123", CreatedAt: 1785342602}
	ts = extractTimestamp(story, am, item)
	if !ts.Equal(postTime) {
		t.Errorf("expected timestamp from story %v, got %v", postTime, ts)
	}

	// 4. Timestamp from PostInfo
	pInfo := posts.PostInfo{ID: "123", CreatedAt: 1785342602}
	ts = extractTimestamp(pInfo, am, item)
	if !ts.Equal(postTime) {
		t.Errorf("expected timestamp from PostInfo %v, got %v", postTime, ts)
	}

	// 5. Fallback to AccountMedia when contentSource has no timestamp
	ts = extractTimestamp(nil, am, item)
	if !ts.Equal(mediaTime) {
		t.Errorf("expected timestamp from AccountMedia %v, got %v", mediaTime, ts)
	}

	// 6. Fallback to MediaItem when AccountMedia has no timestamp
	amNoTs := posts.AccountMedia{ID: "456", CreatedAt: 0}
	itemTime := time.Unix(1785342500, 0)
	ts = extractTimestamp(nil, amNoTs, item)
	if !ts.Equal(itemTime) {
		t.Errorf("expected timestamp from MediaItem %v, got %v", itemTime, ts)
	}
}

func TestApplyFileTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_output.jpg")

	// Create dummy file
	if err := os.WriteFile(filePath, []byte("dummy data"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	targetTime := time.Date(2025, 5, 20, 14, 30, 0, 0, time.UTC)
	applyFileTimestamp(filePath, targetTime)

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	// Compare Unix timestamps to avoid sub-second / OS precision discrepancies
	if info.ModTime().Unix() != targetTime.Unix() {
		t.Errorf("expected mod time unix %d, got %d", targetTime.Unix(), info.ModTime().Unix())
	}
}
