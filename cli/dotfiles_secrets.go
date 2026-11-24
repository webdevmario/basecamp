package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

// SecretFinding says where a secret-looking value is. It never carries the value itself.
type SecretFinding struct {
	Line int    `json:"line"`
	Rule string `json:"rule"`
}

// allowMarker on a line tells the scanner that line is a known false positive.
const allowMarker = "basecamp:allow"

type secretRule struct {
	name string
	re   *regexp.Regexp
	// group, when set, is the submatch that holds the value; values that are
	// variable references ($FOO, ${FOO}, $(cmd)) are not secrets.
	group int
}

var secretRules = []secretRule{
	{name: "private key", re: regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY`)},
	{name: "AWS access key", re: regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{name: "GitHub token", re: regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,})`)},
	{name: "Slack token", re: regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{name: "API key (sk-)", re: regexp.MustCompile(`\bsk-(ant-|proj-)?[A-Za-z0-9_-]{20,}`)},
	{name: "Google API key", re: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{name: "npm auth", re: regexp.MustCompile(`(_authToken|_auth|_password)\s*=\s*(\S+)`), group: 2},
	{name: "registry auth", re: regexp.MustCompile(`"auth"\s*:\s*"([^"]+)"`), group: 1},
	{name: "URL with password", re: regexp.MustCompile(`[a-z][a-z0-9+.-]*://[^/\s:@]+:([^/\s@]+)@`), group: 1},
	{
		// FOO_TOKEN=abc..., "apiKey": "abc...", password: abc...
		name:  "credential assignment",
		re:    regexp.MustCompile(`(?i)[A-Za-z0-9_.-]*(token|secret|passw(or)?d|api[_-]?key|access[_-]?key|private[_-]?key)[A-Za-z0-9_.-]*["']?\s*[=:]\s*["']?([^\s"',;}]{8,})`),
		group: 3,
	},
}

// scanSecrets reports every line that looks like it holds a credential.
func scanSecrets(content []byte) []SecretFinding {
	var out []SecretFinding
	for i, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, allowMarker) {
			continue
		}
		for _, r := range secretRules {
			if ruleMatches(r, line) {
				out = append(out, SecretFinding{Line: i + 1, Rule: r.name})
				break
			}
		}
	}
	return out
}

func ruleMatches(r secretRule, line string) bool {
	if r.group == 0 {
		return r.re.MatchString(line)
	}
	for _, m := range r.re.FindAllStringSubmatch(line, -1) {
		if !isReference(m[r.group]) {
			return true
		}
	}
	return false
}

// isReference is true for values that point somewhere else instead of holding a secret.
func isReference(v string) bool {
	v = strings.Trim(v, `"'`)
	switch {
	case v == "":
		return true
	case strings.HasPrefix(v, "$"), strings.HasPrefix(v, "`"), strings.HasPrefix(v, "<"):
		return true // $FOO, ${env:FOO}, $(security ...), `cmd`, <placeholder>
	case strings.HasPrefix(v, "op://"), strings.HasPrefix(v, "keychain"):
		return true
	}
	lower := strings.ToLower(v)
	for _, w := range []string{"true", "false", "null", "none", "changeme", "xxxxxxxx", "redacted"} {
		if lower == w {
			return true
		}
	}
	return false
}

// forbiddenPath is true for files that are secrets by nature and are never copied,
// whatever their contents look like.
func forbiddenPath(path string) bool {
	base := filepath.Base(path)
	dir := filepath.Base(filepath.Dir(path))
	switch {
	case dir == ".ssh" && base != "config":
		return true // keys, known_hosts, agent sockets
	case strings.HasPrefix(base, "id_") && !strings.HasSuffix(base, ".pub"):
		return true
	case strings.HasSuffix(base, ".pem"), strings.HasSuffix(base, ".key"), strings.HasSuffix(base, ".p12"):
		return true
	case base == ".netrc", base == "credentials", base == ".pgpass", base == ".git-credentials":
		return true
	case dir == "gh" && base == "hosts.yml":
		return true
	}
	return false
}
