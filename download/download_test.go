package download

import (
	"io"
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

func TestProgressWriterQuietMode(t *testing.T) {
	d := &Downloader{}

	if w := d.progressWriter(); w != os.Stderr {
		t.Errorf("expected stderr writer by default, got %T", w)
	}
	d.SetQuiet(true)
	if w := d.progressWriter(); w != io.Discard {
		t.Errorf("expected io.Discard in quiet mode, got %T", w)
	}
	if !d.isQuiet() {
		t.Errorf("expected isQuiet to be true after SetQuiet(true)")
	}
	d.SetQuiet(false)
	if w := d.progressWriter(); w != os.Stderr {
		t.Errorf("expected stderr writer after SetQuiet(false), got %T", w)
	}
}

func TestProgressStatusTracking(t *testing.T) {
	d := &Downloader{}

	d.setProgressTotal(10)
	d.describe("[green]Downloading[reset] 867383099653111808.mp4")
	d.addProgress(1)
	d.addProgress(2)

	st := d.ProgressStatus()
	if st.Total != 10 {
		t.Errorf("expected total 10, got %d", st.Total)
	}
	if st.Current != 3 {
		t.Errorf("expected current 3, got %d", st.Current)
	}
	if st.Description != "Downloading 867383099653111808.mp4" {
		t.Errorf("expected color tags stripped from description, got %q", st.Description)
	}

	// New operation resets the counter
	d.setProgressTotal(5)
	st = d.ProgressStatus()
	if st.Total != 5 || st.Current != 0 {
		t.Errorf("expected reset to 0/5, got %d/%d", st.Current, st.Total)
	}
}

func TestProgressStatusLevel(t *testing.T) {
	d := &Downloader{}
	d.describe("[green]Downloading[reset] a.mp4")
	if st := d.ProgressStatus(); st.Level != "info" {
		t.Errorf("expected info level, got %q", st.Level)
	}
	d.describe("[yellow]Skipping[reset] b.mp4 due to filter")
	if st := d.ProgressStatus(); st.Level != "warn" {
		t.Errorf("expected warn level, got %q", st.Level)
	}
	d.describe("[red]File Exists[reset], Skipping c.mp4")
	if st := d.ProgressStatus(); st.Level != "error" {
		t.Errorf("expected error level, got %q", st.Level)
	}
	if st := d.ProgressStatus(); st.Description != "File Exists, Skipping c.mp4" {
		t.Errorf("expected tags stripped, got %q", st.Description)
	}
}

func TestByteWriterReportsProgress(t *testing.T) {
	d := &Downloader{}
	var lastWritten, lastTotal int64
	bw := &byteWriter{total: 100, report: func(w, tot int64) { lastWritten, lastTotal = w, tot }}
	bw.lastUpd = time.Now().Add(-time.Second) // force first report
	if n, err := bw.Write(make([]byte, 40)); n != 40 || err != nil {
		t.Fatalf("Write returned %d, %v", n, err)
	}
	st := d.ProgressStatus()
	_ = st
	// report goes through the callback, not the status, when wired directly;
	// via d.setBytes the status is updated:
	bw2 := &byteWriter{total: 100, report: d.setBytes}
	bw2.lastUpd = time.Now().Add(-time.Second)
	bw2.Write(make([]byte, 40))
	if st := d.ProgressStatus(); st.Bytes != 40 || st.BytesTotal != 100 {
		t.Errorf("expected 40/100 bytes, got %d/%d", st.Bytes, st.BytesTotal)
	}
	if lastWritten != 40 || lastTotal != 100 {
		t.Errorf("callback got %d/%d", lastWritten, lastTotal)
	}
}

func TestResetProgressClearsState(t *testing.T) {
	d := &Downloader{}
	d.setProgressTotal(13)
	d.describe("[green]Downloading[reset] x.mp4")
	d.addProgress(13)
	d.setBytes(500, 1000)

	d.resetProgress()
	st := d.ProgressStatus()
	if st.Total != 0 || st.Current != 0 || st.Description != "" || st.Bytes != 0 || st.BytesTotal != 0 || st.Level != "" {
		t.Errorf("expected fully cleared status, got %+v", st)
	}
}
