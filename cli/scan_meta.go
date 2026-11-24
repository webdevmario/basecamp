package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func scanMeta() Meta {
	// "Shady-Mac-mini", not the DHCP-flavored "Shady-Mac-mini.attlocal.net"
	hostname := run("scutil", "--get", "LocalHostName")
	if hostname == "" {
		hostname, _ = os.Hostname()
		hostname, _, _ = strings.Cut(hostname, ".")
	}

	osVersion := run("sw_vers", "-productVersion")
	osBuild := run("sw_vers", "-buildVersion")
	major, _, _ := strings.Cut(osVersion, ".")
	osName := "macOS " + major
	if name, ok := macOSNames[major]; ok {
		osName = "macOS " + name
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

// macOSNames maps major versions to marketing names. Unknown versions show as "macOS 27".
var macOSNames = map[string]string{
	"26": "Tahoe",
	"15": "Sequoia",
	"14": "Sonoma",
	"13": "Ventura",
	"12": "Monterey",
}
