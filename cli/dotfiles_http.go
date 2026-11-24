package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

// registerDotfilesRoutes adds the Configs tab's API to a server mux:
//
//	GET  /api/dotfiles        status of every config, with diffs
//	POST /api/dotfiles/save   save changed configs, commit and push
//
// Responses hold config contents (diffs), so only serve them on localhost or a tailnet.
func registerDotfilesRoutes(mux *http.ServeMux) {
	registerDotfilesRoutesWith(mux, defaultDotEnv)
}

func registerDotfilesRoutesWith(mux *http.ServeMux, newEnv func() *dotEnv) {
	var mu sync.Mutex

	mux.HandleFunc("GET /api/dotfiles", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		st := buildStatus(newEnv(), true)
		mu.Unlock()
		dotWriteJSON(w, http.StatusOK, st)
	})

	mux.HandleFunc("POST /api/dotfiles/save", func(w http.ResponseWriter, r *http.Request) {
		// A custom header forces a CORS preflight, which this server never answers,
		// so other web pages cannot trigger a save.
		if r.Header.Get("X-Basecamp") == "" {
			dotWriteJSON(w, http.StatusForbidden, map[string]string{"error": "missing X-Basecamp header"})
			return
		}
		if !mu.TryLock() {
			dotWriteJSON(w, http.StatusConflict, map[string]string{"error": "a save is already running"})
			return
		}
		defer mu.Unlock()
		push := r.URL.Query().Get("push") != "0"
		res, err := saveConfigs(newEnv(), push)
		if err != nil {
			dotWriteJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "result": res})
			return
		}
		dotWriteJSON(w, http.StatusOK, res)
	})
}

func dotWriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
