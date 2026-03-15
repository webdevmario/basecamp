package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// run executes a command and returns trimmed stdout, or empty string on error
func run(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runLines executes a command and returns stdout split by newlines, filtered empty
func runLines(name string, args ...string) []string {
	raw := run(name, args...)
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	var result []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// commandExists checks if a command is available in PATH
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	path = expandHome(path)
	_, err := os.Stat(path)
	return err == nil
}

// expandHome replaces ~ with the actual home directory
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// readFileLines reads a file and returns non-empty lines
func readFileLines(path string) []string {
	path = expandHome(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var result []string
	for _, l := range lines {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}

// countFileLines returns line count of a file
func countFileLines(path string) int {
	return len(readFileLines(path))
}

// fileModTime returns the last modified time of a file, or zero time
func fileModTime(path string) time.Time {
	path = expandHome(path)
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// daysSince returns days between a time and now
func daysSince(t time.Time) int {
	if t.IsZero() {
		return -1
	}
	return int(time.Since(t).Hours() / 24)
}
