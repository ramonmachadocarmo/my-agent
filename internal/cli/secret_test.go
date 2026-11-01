package cli

import (
	"os"
	"strings"
	"testing"

	"my-agent/internal/config"
)

// A key pasted into api_key_env (a field for a variable NAME) must produce a
// clear warning and must never be printed anywhere.
const pastedKey = "AQ.Ab8RN6KoTESTKEYnotreal_0O5rGYmtCw"

func writeCfgWithPastedKey(t *testing.T, path string) {
	t.Helper()
	c := config.Default()
	g := c.Providers["gemini"]
	g.APIKeyEnv = pastedKey
	c.Providers["gemini"] = g
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
}

func TestPastedKeyInApiKeyEnvIsFlaggedAndNeverPrinted(t *testing.T) {
	cfg, _ := setup(t)
	writeCfgWithPastedKey(t, cfg)

	for _, args := range [][]string{
		{"providers", "list"},
		{"config", "validate"},
		{"config", "show"},
		{"config", "get", "providers.gemini.api_key_env"},
	} {
		code, out, errOut := run(t, "", append([]string{"-config", cfg}, args...)...)
		all := out + errOut
		if code != 0 {
			t.Fatalf("%v exited %d: %s", args, code, all)
		}
		if strings.Contains(all, pastedKey) || strings.Contains(all, "AQ.Ab8RN6") {
			t.Fatalf("%v leaked the key:\n%s", args, all)
		}
	}

	out := mustRun(t, "", "-config", cfg, "providers", "list")
	if !strings.Contains(out, "warning: providers.gemini.api_key_env is not a valid environment variable name") ||
		!strings.Contains(out, "providers.gemini.api_key") {
		t.Fatalf("warning missing or unhelpful:\n%s", out)
	}
	if out := mustRun(t, "", "-config", cfg, "config", "validate"); !strings.Contains(out, "config is valid") || !strings.Contains(out, "warning:") {
		t.Fatalf("validate should stay valid but warn:\n%s", out)
	}
	if out := mustRun(t, "", "-config", cfg, "config", "show"); !strings.Contains(out, config.MaskedKey) {
		t.Fatalf("show should mask the misplaced key:\n%s", out)
	}

	// and the fix path works on a config in this state
	mustRun(t, "", "-config", cfg, "config", "set", "providers.gemini.api_key_env", "GEMINI_API_KEY")
	if out := mustRun(t, "", "-config", cfg, "providers", "list"); strings.Contains(out, "warning") {
		t.Fatalf("warning should be gone after the fix:\n%s", out)
	}
	raw, _ := os.ReadFile(cfg)
	if strings.Contains(string(raw), pastedKey) {
		t.Fatal("the pasted key must be gone from the file after being overwritten")
	}
}

func TestNoWarningsForNormalConfig(t *testing.T) {
	cfg, _ := setup(t)
	mustRun(t, "", "-config", cfg, "init")
	if out := mustRun(t, "", "-config", cfg, "providers", "list"); strings.Contains(out, "warning") {
		t.Fatalf("default config must not warn:\n%s", out)
	}
}
