package main

import "testing"

func TestScanSecretsFinds(t *testing.T) {
	// Built from pieces so this file never looks like it holds a real token.
	gh := "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"
	cases := map[string]string{
		"export GITHUB_TOKEN=" + gh:                               "GitHub token",
		"//registry.npmjs.org/:_authToken=npm_abc123def456":       "npm auth",
		`{"auths":{"ghcr.io":{"auth":"dXNlcjpwYXNz"}}}`:           "registry auth",
		"export OPENAI_API_KEY=sk-" + "proj-abcdefghij1234567890": "API key (sk-)",
		"export AWS_ACCESS_KEY_ID=AKIA" + "ABCDEFGHIJKLMNOP":      "AWS access key",
		"-----BEGIN OPENSSH " + "PRIVATE KEY-----":                "private key",
		"export DB_PASSWORD=hunter2hunter2":                       "credential assignment",
		`"apiKey": "abcd1234efgh5678"`:                            "credential assignment",
		"url = https://user:s3cretpass@example.com/repo.git":      "URL with password",
	}
	for line, rule := range cases {
		got := scanSecrets([]byte("# header\n" + line + "\n"))
		if len(got) != 1 || got[0].Line != 2 || got[0].Rule != rule {
			t.Errorf("%q: got %+v, want %s on line 2", line, got, rule)
		}
	}
}

func TestScanSecretsIgnores(t *testing.T) {
	clean := []string{
		"export NVM_DIR=\"$HOME/.nvm\"",
		"export GITHUB_TOKEN=$(security find-generic-password -s gh -w)",
		"//registry.npmjs.org/:_authToken=${NPM_TOKEN}",
		`"apiKey": "${env:OPENAI_API_KEY}"`,
		`"editor.semanticTokenColorCustomizations": {`,
		`"github.copilot.enable": {"*": true},`,
		"token_limit = 4096",
		"[credential]\n\thelper = osxkeychain",
		"export API_KEY=sk-" + "abcdefghijklmnopqrstuvwxyz # basecamp:allow",
		"IdentityFile ~/.ssh/id_ed25519",
		`"password": "op://Private/db/password"`,
	}
	for _, line := range clean {
		if got := scanSecrets([]byte(line)); len(got) != 0 {
			t.Errorf("%q: false positive %+v", line, got)
		}
	}
}

func TestForbiddenPath(t *testing.T) {
	for p, want := range map[string]bool{
		"/h/.ssh/config":           false,
		"/h/.ssh/id_ed25519":       true,
		"/h/.ssh/id_ed25519.pub":   true, // nothing in .ssh but config is copied
		"/h/.ssh/known_hosts":      true,
		"/h/certs/server.pem":      true,
		"/h/.netrc":                true,
		"/h/.aws/credentials":      true,
		"/h/.config/gh/hosts.yml":  true,
		"/h/.config/gh/config.yml": false,
		"/h/.zshrc":                false,
	} {
		if got := forbiddenPath(p); got != want {
			t.Errorf("forbiddenPath(%q) = %v, want %v", p, got, want)
		}
	}
}
