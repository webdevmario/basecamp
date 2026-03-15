package main

import (
	"fmt"
	"os"
	"time"
)

func scanMeta() Meta {
	hostname, _ := os.Hostname()

	osVersion := run("sw_vers", "-productVersion")
	osBuild := run("sw_vers", "-buildVersion")
	osName := "macOS"
	// Map version to name
	if len(osVersion) >= 2 {
		major := osVersion[:2]
		switch major {
		case "15":
			osName = "macOS Sequoia"
		case "14":
			osName = "macOS Sonoma"
		case "13":
			osName = "macOS Ventura"
		case "12":
			osName = "macOS Monterey"
		}
	}

	chip := run("sysctl", "-n", "machdep.cpu.brand_string")
	if chip == "" {
		// Apple Silicon
		chip = run("sysctl", "-n", "hw.chip")
		if chip == "" {
			chip = "Unknown"
		}
	}

	// Memory in GB
	memBytes := run("sysctl", "-n", "hw.memsize")
	memory := "Unknown"
	if memBytes != "" {
		var bytes int64
		fmt.Sscanf(memBytes, "%d", &bytes)
		memory = fmt.Sprintf("%d GB", bytes/(1024*1024*1024))
	}

	shell := os.Getenv("SHELL")
	if shell != "" {
		shellVersion := run(shell, "--version")
		if shellVersion != "" {
			// Try to extract just the version part
			shell = shell + " (" + shellVersion + ")"
		}
	}

	return Meta{
		Hostname:  hostname,
		OS:        osName,
		OSVersion: fmt.Sprintf("%s (%s)", osVersion, osBuild),
		Chip:      chip,
		Memory:    memory,
		Shell:     shell,
		LastScan:  time.Now().UTC().Format(time.RFC3339),
	}
}
