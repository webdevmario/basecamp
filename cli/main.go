package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: basecamp <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  scan    Scan the current environment (-o file, -pretty)")
	fmt.Fprintln(os.Stderr, "  serve   Serve the dashboard and the latest scan (-addr, -web, -data)")
	fmt.Fprintln(os.Stderr, "  dotfiles  Keep configs saved in a private repo (status, diff, save, restore)")
	fmt.Fprintln(os.Stderr, "  jobs    Check scheduled jobs (LaunchAgents) and backups into ~/.basecamp/jobs.json (-h for flags)")
}

func main() {
	// Flags before the command (basecamp -o file scan) still work, for older scripts.
	pretty := flag.Bool("pretty", false, "Pretty-print JSON output")
	output := flag.String("o", "", "Write output to file instead of stdout")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	switch args[0] {
	case "scan":
		fs := flag.NewFlagSet("scan", flag.ExitOnError)
		p := fs.Bool("pretty", *pretty, "Pretty-print JSON output")
		o := fs.String("o", *output, "Write output to file instead of stdout")
		fs.Parse(args[1:])
		runScan(*p, *o)
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		addr := fs.String("addr", ":3050", "Address to listen on")
		web := fs.String("web", "", "Built dashboard directory (web/dist)")
		data := fs.String("data", defaultDataDir(), "Directory holding scan.json")
		fs.Parse(args[1:])
		runServe(*addr, *web, *data)
	case "dotfiles":
		runDotfiles(args[1:])
	case "jobs":
		runJobs(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		os.Exit(1)
	}
}

// defaultDataDir is where scans live outside the repo: ~/.basecamp
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".basecamp"
	}
	return filepath.Join(home, ".basecamp")
}

func collectScan() ScanResult {
	return ScanResult{
		Meta: scanMeta(),
		Categories: []Category{
			scanDotfiles(),
			scanVSCode(),
			scanRuntimes(),
			scanGlobalPackages(),
			scanFonts(),
			scanMacOSDefaults(),
			scanSSHKeys(),
			scanServices(),
		},
	}
}

func runScan(pretty bool, outputPath string) {
	fmt.Fprintln(os.Stderr, "🔍 Scanning environment...")
	start := time.Now()

	result := collectScan()

	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "✅ Scan complete in %s, %d categories, %d items\n",
		elapsed.Round(time.Millisecond), len(result.Categories), result.TotalItems())

	var data []byte
	var err error
	if pretty {
		data, err = json.MarshalIndent(result, "", "  ")
	} else {
		data, err = json.Marshal(result)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if outputPath != "" {
		if err := writeFileAtomic(outputPath, data); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "📄 Written to %s\n", outputPath)
	} else {
		fmt.Println(string(data))
	}
}

// writeFileAtomic writes via a temp file and rename, so a reader never sees half a scan.
func writeFileAtomic(path string, data []byte) error {
	path = expandHome(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".scan-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	os.Chmod(tmp.Name(), 0o644)
	return os.Rename(tmp.Name(), path)
}
