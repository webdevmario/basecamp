package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// runServe serves the built dashboard and the latest scan from dataDir.
//
//	GET  /scan.json    latest scan (dataDir/scan.json)
//	POST /api/scan     run a scan now, save it, return it
//	GET  /api/health   liveness plus when the last scan ran
//	GET  /*            the dashboard (webDir)
func runServe(addr, webDir, dataDir string) {
	dataDir = expandHome(dataDir)
	scanPath := filepath.Join(dataDir, "scan.json")
	var scanMu sync.Mutex

	mux := http.NewServeMux()

	mux.HandleFunc("GET /scan.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, scanPath)
	})

	mux.HandleFunc("POST /api/scan", func(w http.ResponseWriter, r *http.Request) {
		if !scanMu.TryLock() {
			http.Error(w, "a scan is already running", http.StatusConflict)
			return
		}
		defer scanMu.Unlock()
		start := time.Now()
		result := collectScan()
		data, err := json.Marshal(result)
		if err == nil {
			err = writeFileAtomic(scanPath, data)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("scan via API: %d items in %s", result.TotalItems(), time.Since(start).Round(time.Millisecond))
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"ok": true}
		if info, err := os.Stat(scanPath); err == nil {
			resp["scanUpdated"] = info.ModTime().UTC().Format(time.RFC3339)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	if webDir != "" {
		webDir = expandHome(webDir)
		files := http.FileServer(http.Dir(webDir))
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			p := filepath.Join(webDir, filepath.Clean("/"+r.URL.Path))
			if info, err := os.Stat(p); err != nil || info.IsDir() {
				// Unknown paths and "/" get the app shell.
				w.Header().Set("Cache-Control", "no-cache")
				http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
				return
			}
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
		})
	}

	fmt.Fprintf(os.Stderr, "basecamp serving on %s (web: %q, data: %s), tailnet and localhost only\n", addr, webDir, dataDir)
	log.Fatal(http.ListenAndServe(addr, tailnetOnly(mux)))
}

// Tailscale hands out 100.64.0.0/10 and fd7a:115c:a1e0::/48. Anything else (the home LAN,
// a forwarded port) gets a 403: scans and config diffs never leave the tailnet.
var allowedNets = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("fd7a:115c:a1e0::/48"),
}

func tailnetOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ap, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
			ip := ap.Addr().Unmap()
			ok := ip.IsLoopback()
			for _, p := range allowedNets {
				ok = ok || p.Contains(ip)
			}
			if ok {
				next.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, "basecamp is only reachable over the tailnet", http.StatusForbidden)
	})
}
