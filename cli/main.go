package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	// Flags
	pretty := flag.Bool("pretty", false, "Pretty-print JSON output")
	output := flag.String("o", "", "Write output to file instead of stdout")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: basecamp <command> [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  scan    Scan the current environment")
		os.Exit(1)
	}

	switch args[0] {
	case "scan":
		runScan(*pretty, *output)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		os.Exit(1)
	}
}

func runScan(pretty bool, outputPath string) {
	fmt.Fprintln(os.Stderr, "🔍 Scanning environment...")
	start := time.Now()

	result := ScanResult{
		Meta: scanMeta(),
		Categories: []Category{
			scanDotfiles(),
			scanHomebrew(),
			scanVSCode(),
			scanRuntimes(),
			scanGlobalPackages(),
			scanFonts(),
			scanMacOSDefaults(),
			scanSSHKeys(),
			scanServices(),
		},
	}

	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "✅ Scan complete in %s — %d categories, %d items\n",
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
		err = os.WriteFile(outputPath, data, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "📄 Written to %s\n", outputPath)
	} else {
		fmt.Println(string(data))
	}
}
