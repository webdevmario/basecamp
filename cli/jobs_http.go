package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// registerJobsRoutes serves what `basecamp jobs` writes:
//
//	GET /jobs.json         the full report (dataDir/jobs.json) for the Jobs tab
//	GET /api/jobs/status   one green/red answer for other dashboards (Springboard); CORS open,
//	                       still behind the tailnet-only guard
func registerJobsRoutes(mux *http.ServeMux, dataDir string) {
	jobsPath := filepath.Join(dataDir, "jobs.json")
	mux.HandleFunc("GET /jobs.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, jobsPath)
	})
	mux.HandleFunc("GET /api/jobs/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobsStatus(jobsPath, time.Now()))
	})
}

// JobsStatus is the one-line answer Springboard shows as a dot.
type JobsStatus struct {
	OK          bool     `json:"ok"`
	Red         int      `json:"red"`
	Warn        int      `json:"warn"`
	Total       int      `json:"total"`
	RedJobs     []string `json:"redJobs"`
	GeneratedAt string   `json:"generatedAt,omitempty"`
	Message     string   `json:"message"`
}

// jobsStatus reads jobs.json. A missing or old report is red too: it means the 15-minute check
// itself stopped.
func jobsStatus(path string, now time.Time) JobsStatus {
	st := JobsStatus{RedJobs: []string{}}
	b, err := os.ReadFile(path)
	if err != nil {
		st.Message = "no jobs report yet (basecamp jobs hasn't run)"
		return st
	}
	var rep JobsReport
	if err := json.Unmarshal(b, &rep); err != nil {
		st.Message = "jobs report is unreadable"
		return st
	}
	st.GeneratedAt, st.Red, st.Warn, st.Total = rep.GeneratedAt, rep.Summary.Red, rep.Summary.Warn, rep.Summary.Total
	for _, j := range rep.Jobs {
		if j.Red {
			st.RedJobs = append(st.RedJobs, j.Label)
		}
	}
	gen := parseAnyTime(rep.GeneratedAt, time.Local)
	switch {
	case gen.IsZero() || now.Sub(gen) > time.Hour:
		st.Message = "jobs report is over an hour old (is com.basecamp-jobs running?)"
	case st.Red > 0:
		st.Message = fmt.Sprintf("%d of %d jobs need attention", st.Red, st.Total)
	default:
		st.OK = true
		st.Message = fmt.Sprintf("all %d jobs fine", st.Total)
		if st.Warn > 0 {
			st.Message += fmt.Sprintf(", %d with warnings", st.Warn)
		}
	}
	return st
}
