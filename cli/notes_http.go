package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// registerNotesRoutes keeps the per-item notes in dataDir/notes.json, so they are the
// same on every device instead of living in one browser's localStorage.
//
//	GET /api/notes   {"categoryId::itemName": "note", ...}
//	PUT /api/notes   replaces the whole map
func registerNotesRoutes(mux *http.ServeMux, dataDir string) {
	path := filepath.Join(dataDir, "notes.json")
	var mu sync.Mutex

	mux.HandleFunc("GET /api/notes", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		data, err := os.ReadFile(path)
		mu.Unlock()
		if os.IsNotExist(err) {
			data, err = []byte("{}"), nil
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
	})

	mux.HandleFunc("PUT /api/notes", func(w http.ResponseWriter, r *http.Request) {
		var notes map[string]string
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&notes); err != nil || notes == nil {
			http.Error(w, "expected a JSON object of string notes", http.StatusBadRequest)
			return
		}
		data, _ := json.MarshalIndent(notes, "", "  ")
		mu.Lock()
		err := writeFileAtomic(path, data)
		mu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
