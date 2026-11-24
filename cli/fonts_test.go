package main

import "testing"

func TestExtractFontFamily(t *testing.T) {
	cases := map[string]string{
		"JetBrainsMono-ExtraBoldItalic.ttf":  "JetBrainsMono",
		"JetBrainsMono-Italic[wght].ttf":     "JetBrainsMono",
		"JetBrainsMono[wght].ttf":            "JetBrainsMono",
		"JetBrainsMonoNL-ThinItalic.ttf":     "JetBrainsMonoNL",
		"FuraMono-Bold Powerline.otf":        "FuraMono Powerline",
		"Anonymous Pro BI.ttf":               "Anonymous Pro",
		"Anonymous Pro.ttf":                  "Anonymous Pro",
		"FiraCode-Retina.ttf":                "FiraCode",
		"DepartureMono-Regular.otf":          "DepartureMono",
		"Arial Unicode.ttf":                  "Arial Unicode",
		"SF-Mono-Semibold.otf":               "SF-Mono",
	}
	for in, want := range cases {
		if got := extractFontFamily(in); got != want {
			t.Errorf("extractFontFamily(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{"png": "png", "~/Desktop/Shots": "'~/Desktop/Shots'", "a b": "'a b'", "it's": `'it'\''s'`}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
