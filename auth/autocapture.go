package auth

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "sync"
)

// CapturedInfo carries auth token and user agent captured from the browser.
type CapturedInfo struct {
    Token     string
    UserAgent string
}

// StartAutoCaptureServer starts a local HTTP server on 127.0.0.1 that listens
// for a GET request at /capture?token=...&ua=... and returns the chosen port,
// a channel where the first captured credentials are delivered, and a shutdown
// function to stop the server gracefully.
func StartAutoCaptureServer() (port int, ch <-chan CapturedInfo, shutdown func(context.Context) error, err error) {
    mux := http.NewServeMux()

    outCh := make(chan CapturedInfo, 1)
    var once sync.Once

    mux.HandleFunc("/capture", func(w http.ResponseWriter, r *http.Request) {
        q := r.URL.Query()
        token := q.Get("token")
        ua := q.Get("ua")

        if token == "" || ua == "" {
            w.WriteHeader(http.StatusBadRequest)
            _, _ = w.Write([]byte("missing token or ua"))
            return
        }

        once.Do(func() {
            outCh <- CapturedInfo{Token: token, UserAgent: ua}
            close(outCh)
        })

        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.Header().Set("Cache-Control", "no-store")
        _, _ = w.Write([]byte("OK"))
    })

    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // A minimal landing page with instructions.
        // Users typically won’t open this; the TUI shows instructions/snippet.
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        _, _ = w.Write([]byte(`<html><head><title>Fansly Scraper Capture</title></head><body>
<h3>Fansly Scraper: Token Capture</h3>
<p>This page is used by the scraper to receive your token from your browser.</p>
<p>Open fansly.com, log in, open DevTools Console and run the snippet shown in the scraper.</p>
</body></html>`))
    })

    srv := &http.Server{Handler: mux}

    // Listen on 127.0.0.1:0 (auto-assign free port)
    ln, e := net.Listen("tcp", "127.0.0.1:0")
    if e != nil {
        return 0, nil, nil, e
    }

    // Extract the chosen port
    addr := ln.Addr().String()
    // addr like 127.0.0.1:54321
    var p int
    if _, e := fmt.Sscanf(addr, "127.0.0.1:%d", &p); e != nil {
        // fallback: parse by splitting
        host, portStr, _ := net.SplitHostPort(addr)
        if host == "" || portStr == "" {
            _ = ln.Close()
            return 0, nil, nil, e
        }
        var pe error
        _, pe = fmt.Sscanf(portStr, "%d", &p)
        if pe != nil {
            _ = ln.Close()
            return 0, nil, nil, pe
        }
    }

    go func() {
        _ = srv.Serve(ln)
    }()

    shutdown = func(ctx context.Context) error {
        return srv.Shutdown(ctx)
    }

    return p, outCh, shutdown, nil
}

