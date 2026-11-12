package download

import (
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/schollz/progressbar/v3"
)

// Simulates the TUI polling ProgressStatus() while download goroutines
// call describe/addProgress/setProgressTotal concurrently against a real
// progress bar (as in production).
func TestProgressStatusConcurrent(t *testing.T) {
	d := &Downloader{}
	d.SetQuiet(true)
	d.progressBar = progressbar.NewOptions(100,
		progressbar.OptionSetWriter(io.Discard),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
	)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				d.setProgressTotal(100)
				d.describe(fmt.Sprintf("[green]Downloading[reset] file_%d.mp4", i))
				d.addProgress(1)
			}
		}(g)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = d.ProgressStatus()
			_ = d.progressWriter()
			d.SetQuiet(i%2 == 0)
		}
	}()
	wg.Wait()

	st := d.ProgressStatus()
	if st.Description == "" {
		t.Errorf("expected description to be set")
	}
}
