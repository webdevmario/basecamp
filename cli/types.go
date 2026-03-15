package main

// ScanResult is the top-level output of a basecamp scan
type ScanResult struct {
	Meta       Meta       `json:"meta"`
	Categories []Category `json:"categories"`
}

func (s ScanResult) TotalItems() int {
	total := 0
	for _, c := range s.Categories {
		total += len(c.Items)
	}
	return total
}

type Meta struct {
	Hostname  string `json:"hostname"`
	OS        string `json:"os"`
	OSVersion string `json:"osVersion"`
	Chip      string `json:"chip"`
	Memory    string `json:"memory"`
	Shell     string `json:"shell"`
	LastScan  string `json:"lastScan"`
}

type Category struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Desc  string `json:"desc"`
	Items []Item `json:"items"`
}

type Item struct {
	Name       string `json:"name"`
	Detail     string `json:"detail,omitempty"`
	Status     string `json:"status"` // active, current, running, outdated, stale
	SystemNote string `json:"systemNote,omitempty"`
	UserNote   string `json:"userNote,omitempty"`

	// For runtimes with version managers
	VersionManager string          `json:"versionManager,omitempty"`
	Versions       []VersionEntry  `json:"versions,omitempty"`
}

type VersionEntry struct {
	Version string   `json:"version"`
	Label   string   `json:"label,omitempty"` // "current", "lts", etc.
	Globals []string `json:"globals"`         // "package@version" format
}
