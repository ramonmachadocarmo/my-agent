package cli

import (
	"os"
	"testing"

	"my-agent/internal/i18n"
)

// The tests assert English messages, so they must not depend on the language
// of the machine they run on. Tests of other languages select them explicitly
// (with --lang or the config's "language").
func TestMain(m *testing.M) {
	os.Setenv(i18n.EnvVar, "en")
	i18n.Set(i18n.EN)
	os.Exit(m.Run())
}
