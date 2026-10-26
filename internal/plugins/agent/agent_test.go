package agent

import "testing"

// TestAgentName finds the agents the plugin watches in real command lines,
// and does not mistake other programs for one.
func TestAgentName(t *testing.T) {
	matched := map[string]string{
		"claude --resume": "Claude Code",
		`"C:\Users\ada\AppData\Roaming\npm\claude.cmd"`: "Claude Code",
		"codex exec --full-auto":                        "Codex",
		"/usr/local/bin/codex":                          "Codex",
		"aider --model gpt-5":                           "Aider",
		"node /opt/homebrew/bin/gemini":                 "Gemini CLI",
		"opencode serve":                                "opencode",
	}
	for command, want := range matched {
		if got := agentName(command, nil); got != want {
			t.Errorf("agentName(%q) = %q, want %q", command, got, want)
		}
	}

	unmatched := []string{
		"explorer.exe",
		`F:\Project\example\app.exe`,
		"powershell.exe -NoProfile",
		"node ./node_modules/.bin/vite",
	}
	for _, command := range unmatched {
		if got := agentName(command, nil); got != "" {
			t.Errorf("agentName(%q) = %q, want no agent", command, got)
		}
	}
}

// TestAgentNameExtras lets the user watch an agent of their own.
func TestAgentNameExtras(t *testing.T) {
	if got := agentName("go run ./my-agent --serve", []string{"my-agent"}); got != "my-agent" {
		t.Errorf("extra name = %q", got)
	}
}
