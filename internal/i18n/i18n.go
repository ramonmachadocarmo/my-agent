// Package i18n holds the translations (English, Brazilian Portuguese and
// Spanish) and the language selection used by the CLI, the chat and the web UI.
//
// Every message lives in one catalog keyed by a stable ID, with the three
// translations side by side, so they are reviewed together and tests can
// prove that none is missing. Messages are printf templates: T(key, args...).
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Lang is a supported language.
type Lang string

const (
	EN   Lang = "en"
	PtBR Lang = "pt-BR"
	ES   Lang = "es"
)

// Auto in a configuration means "detect it from the environment".
const Auto = "auto"

// Supported lists the languages in the order shown to the user.
var Supported = []Lang{EN, PtBR, ES}

// Names are the languages' own names, for menus.
var Names = map[Lang]string{EN: "English", PtBR: "Português (Brasil)", ES: "Español"}

// EnvVar overrides the language for one run.
const EnvVar = "MY_AGENT_LANG"

// m is one catalog entry: the message in {English, Portuguese, Spanish}.
type m [3]string

var (
	mu      sync.RWMutex
	current = EN
	catalog = map[string]m{}
)

// add registers messages; used by the msgs_*.go files.
func add(entries map[string]m) {
	for k, v := range entries {
		if _, dup := catalog[k]; dup {
			panic("i18n: duplicate key " + k)
		}
		catalog[k] = v
	}
}

func index(l Lang) int {
	switch l {
	case PtBR:
		return 1
	case ES:
		return 2
	}
	return 0
}

// Set selects the language used by T.
func Set(l Lang) {
	mu.Lock()
	current = l
	mu.Unlock()
}

// Current returns the language used by T.
func Current() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// T returns the message for key in the current language, formatted with args.
// A key missing in the language falls back to English; an unknown key is
// returned as is, so a mistake is visible instead of silent.
func T(key string, args ...any) string { return TL(Current(), key, args...) }

// TL is T for an explicit language.
func TL(l Lang, key string, args ...any) string {
	e, ok := catalog[key]
	if !ok {
		return key
	}
	s := e[index(l)]
	if s == "" {
		s = e[0]
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// N picks the singular (key+".one") or plural (key+".other") form for n and
// formats it with n followed by args.
func N(key string, n int, args ...any) string {
	if n == 1 {
		key += ".one"
	} else {
		key += ".other"
	}
	return T(key, append([]any{n}, args...)...)
}

// Has reports whether key exists.
func Has(key string) bool { _, ok := catalog[key]; return ok }

// Keys returns every key (for tests and tooling).
func Keys() []string {
	out := make([]string, 0, len(catalog))
	for k := range catalog {
		out = append(out, k)
	}
	return out
}

// Raw returns the untranslated template of key in l ("" if missing).
func Raw(l Lang, key string) string {
	e, ok := catalog[key]
	if !ok {
		return ""
	}
	return e[index(l)]
}

// Messages returns every message whose key starts with prefix, in language l
// (English fills the gaps). The web UI loads its texts with it.
func Messages(l Lang, prefix string) map[string]string {
	out := map[string]string{}
	for k, e := range catalog {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		s := e[index(l)]
		if s == "" {
			s = e[0]
		}
		out[k] = s
	}
	return out
}

// Normalize maps a locale name ("pt", "pt_BR.UTF-8", "es-MX", "en-GB") to a
// supported language. Any Portuguese becomes pt-BR and any Spanish es.
func Normalize(s string) (Lang, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if i := strings.IndexAny(s, ".@"); i >= 0 { // strip encoding and modifier
		s = s[:i]
	}
	s = strings.ReplaceAll(strings.ToLower(s), "_", "-")
	base, _, _ := strings.Cut(s, "-")
	switch base {
	case "en":
		return EN, true
	case "pt":
		return PtBR, true
	case "es":
		return ES, true
	}
	return "", false
}

// Detect picks the language. The candidates are tried in order and the first
// that names a supported language wins ("auto" and empty are skipped); then
// the operating system's language; then English. Callers pass, in this
// order: the --lang flag, $MY_AGENT_LANG, the configuration's language.
func Detect(candidates ...string) Lang {
	for _, c := range candidates {
		if l, ok := Normalize(c); ok {
			return l
		}
	}
	return SystemLanguage()
}

// SystemLanguage reads the operating system's UI language.
func SystemLanguage() Lang {
	if s := osLocale(); s != "" {
		if l, ok := Normalize(s); ok {
			return l
		}
	}
	// POSIX order of precedence; LANGUAGE may hold a colon separated list
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		val := os.Getenv(v)
		for _, part := range strings.Split(val, ":") {
			if l, ok := Normalize(part); ok {
				return l
			}
		}
	}
	return EN
}

// Valid reports whether s is an accepted value for a configuration's
// language: empty, "auto" or a supported language.
func Valid(s string) bool {
	if s == "" || strings.EqualFold(s, Auto) {
		return true
	}
	_, ok := Normalize(s)
	return ok
}

// Canonical returns the value to store for a language setting: "auto" or the
// canonical language code.
func Canonical(s string) string {
	if strings.TrimSpace(s) == "" || strings.EqualFold(s, Auto) {
		return Auto
	}
	if l, ok := Normalize(s); ok {
		return string(l)
	}
	return s
}
