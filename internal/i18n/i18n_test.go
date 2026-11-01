package i18n

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var verbRE = regexp.MustCompile(`%(?:\[(\d+)\])?[-+# 0]*(?:\d+|\*)?(?:\.\d+)?([a-zA-Z%])`)

// verbs maps each argument position to its verb letter, following Go's rules
// (an explicit index such as %[2]s moves the position).
func verbs(s string) map[int]string {
	out := map[int]string{}
	pos := 1
	for _, m := range verbRE.FindAllStringSubmatch(s, -1) {
		if m[2] == "%" {
			continue
		}
		if m[1] != "" {
			pos, _ = strconv.Atoi(m[1])
		}
		out[pos] = m[2]
		pos++
	}
	return out
}

func TestEveryKeyIsTranslatedIntoEveryLanguage(t *testing.T) {
	keys := Keys()
	if len(keys) == 0 {
		t.Fatal("empty catalog")
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, l := range Supported {
			if strings.TrimSpace(Raw(l, k)) == "" {
				t.Errorf("%s: missing %s translation", k, l)
			}
		}
	}
}

func TestTranslationsKeepTheSamePlaceholders(t *testing.T) {
	keys := Keys()
	sort.Strings(keys)
	for _, k := range keys {
		en := verbs(Raw(EN, k))
		for _, l := range []Lang{PtBR, ES} {
			got := verbs(Raw(l, k))
			if fmt.Sprint(got) != fmt.Sprint(en) {
				t.Errorf("%s: %s uses placeholders %v but English uses %v\n  en: %s\n  %s: %s", k, l, got, en, Raw(EN, k), l, Raw(l, k))
			}
		}
	}
}

func TestTranslationsAreNotJustCopiesOfEnglishWhereTheyShouldDiffer(t *testing.T) {
	// A message may legitimately be identical (a command name, "OK"...), but a
	// long sentence identical in all three languages was almost certainly forgotten.
	for _, k := range Keys() {
		en := Raw(EN, k)
		if len(en) > 40 && en == Raw(PtBR, k) && en == Raw(ES, k) {
			t.Errorf("%s looks untranslated (same long text in all languages): %q", k, en)
		}
	}
}

func TestKeysHaveAnArea(t *testing.T) {
	for _, k := range Keys() {
		if !strings.Contains(k, ".") || strings.ContainsAny(k, " \t") {
			t.Errorf("bad key %q: use area.name", k)
		}
	}
}

func TestPluralKeysComeInPairs(t *testing.T) {
	for _, k := range Keys() {
		if base, ok := strings.CutSuffix(k, ".one"); ok && !Has(base+".other") {
			t.Errorf("%s has no .other form", k)
		}
		if base, ok := strings.CutSuffix(k, ".other"); ok && !Has(base+".one") {
			t.Errorf("%s has no .one form", k)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]Lang{
		"en": EN, "EN": EN, "en_US.UTF-8": EN, "en-GB": EN,
		"pt": PtBR, "pt-BR": PtBR, "pt_BR.UTF-8": PtBR, "pt-br": PtBR, "pt-PT": PtBR,
		"es": ES, "es-ES": ES, "es_MX.UTF-8": ES, "es-419": ES, "ES@euro": ES,
	}
	for in, want := range cases {
		if got, ok := Normalize(in); !ok || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "  ", "C", "POSIX", "fr", "de-DE", "auto", "english"} {
		if got, ok := Normalize(in); ok {
			t.Errorf("Normalize(%q) = %q; should be unsupported", in, got)
		}
	}
}

func TestDetectPriority(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANGUAGE", "")
	t.Setenv("LANG", "es_ES.UTF-8")
	if osLocale() == "" { // on Unix the environment decides
		if got := Detect(); got != ES {
			t.Errorf("from LANG: %q", got)
		}
	}
	// candidates win over the system, in order; "auto", empty and junk are skipped
	if got := Detect("pt-BR", "en"); got != PtBR {
		t.Errorf("first candidate: %q", got)
	}
	if got := Detect("", "auto", "xx", "en"); got != EN {
		t.Errorf("skips auto/empty/unsupported: %q", got)
	}
}

func TestSystemLanguageReadsEnvironmentPrecedence(t *testing.T) {
	if osLocale() != "" {
		t.Skip("Windows reports the display language through the OS, not the environment")
	}
	set := func(all, msgs, language, lang string) {
		t.Setenv("LC_ALL", all)
		t.Setenv("LC_MESSAGES", msgs)
		t.Setenv("LANGUAGE", language)
		t.Setenv("LANG", lang)
	}
	set("", "", "", "")
	if got := SystemLanguage(); got != EN {
		t.Errorf("nothing set: %q", got)
	}
	set("", "", "", "pt_BR.UTF-8")
	if got := SystemLanguage(); got != PtBR {
		t.Errorf("LANG: %q", got)
	}
	set("es_ES.UTF-8", "", "", "pt_BR.UTF-8")
	if got := SystemLanguage(); got != ES {
		t.Errorf("LC_ALL beats LANG: %q", got)
	}
	set("", "", "fr:pt_BR:en", "en_US")
	if got := SystemLanguage(); got != PtBR {
		t.Errorf("LANGUAGE is a list; first supported wins: %q", got)
	}
	set("C", "", "", "C")
	if got := SystemLanguage(); got != EN {
		t.Errorf("C locale: %q", got)
	}
}

func TestValidAndCanonical(t *testing.T) {
	for in, want := range map[string]string{"": "auto", "AUTO": "auto", "auto": "auto", "pt": "pt-BR", "pt_br": "pt-BR", "es-MX": "es", "EN": "en"} {
		if !Valid(in) || Canonical(in) != want {
			t.Errorf("%q: valid=%v canonical=%q, want %q", in, Valid(in), Canonical(in), want)
		}
	}
	for _, in := range []string{"fr", "klingon", "xx-YY"} {
		if Valid(in) {
			t.Errorf("%q must not be valid", in)
		}
	}
}

func TestTFormatsAndFallsBack(t *testing.T) {
	Set(EN)
	defer Set(EN)
	if got := T("lang.unsupported", "xx"); !strings.Contains(got, `"xx"`) || !strings.Contains(got, "unsupported language") {
		t.Errorf("en: %q", got)
	}
	Set(PtBR)
	if got := T("lang.unsupported", "xx"); !strings.Contains(got, "idioma não suportado") {
		t.Errorf("pt: %q", got)
	}
	if got := TL(ES, "lang.unsupported", "xx"); !strings.Contains(got, "idioma no compatible") {
		t.Errorf("explicit language: %q", got)
	}
	// unknown keys are returned as is, so a typo is visible
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key: %q", got)
	}
	// a key with a missing translation falls back to English
	catalog["test.gap"] = m{"only english", "", ""}
	defer delete(catalog, "test.gap")
	Set(ES)
	if got := T("test.gap"); got != "only english" {
		t.Errorf("fallback: %q", got)
	}
}

func TestTWithoutArgsDoesNotTouchPercentSigns(t *testing.T) {
	catalog["test.pct"] = m{"100% sure", "100% certo", "100% seguro"}
	defer delete(catalog, "test.pct")
	Set(EN)
	if got := T("test.pct"); got != "100% sure" {
		t.Errorf("%q", got)
	}
}

func TestPlurals(t *testing.T) {
	catalog["test.docs.one"] = m{"%d document", "%d documento", "%d documento"}
	catalog["test.docs.other"] = m{"%d documents", "%d documentos", "%d documentos"}
	defer delete(catalog, "test.docs.one")
	defer delete(catalog, "test.docs.other")
	Set(EN)
	defer Set(EN)
	for n, want := range map[int]string{0: "0 documents", 1: "1 document", 2: "2 documents"} {
		if got := N("test.docs", n); got != want {
			t.Errorf("N(%d) = %q, want %q", n, got, want)
		}
	}
	Set(PtBR)
	if got := N("test.docs", 3); got != "3 documentos" {
		t.Errorf("pt: %q", got)
	}
}

func TestErrIsLazyAndUsableAsASentinel(t *testing.T) {
	catalog["test.err"] = m{"it failed", "falhou", "falló"}
	defer delete(catalog, "test.err")
	sentinel := Err("test.err")
	wrapped := fmt.Errorf("context: %w", sentinel)
	if !errors.Is(wrapped, sentinel) {
		t.Fatal("errors.Is must work through wrapping")
	}
	if errors.Is(wrapped, Err("test.err")) {
		t.Fatal("two Err calls are two different sentinels")
	}
	Set(EN)
	defer Set(EN)
	if sentinel.Error() != "it failed" {
		t.Errorf("%q", sentinel.Error())
	}
	Set(ES) // the text follows the language, even for an error created earlier
	if sentinel.Error() != "falló" {
		t.Errorf("%q", sentinel.Error())
	}
}

func TestErrorfKeepsWrapping(t *testing.T) {
	catalog["test.wrap"] = m{"reading %s: %w", "lendo %s: %w", "leyendo %s: %w"}
	defer delete(catalog, "test.wrap")
	Set(PtBR)
	defer Set(EN)
	cause := os.ErrNotExist
	err := Errorf("test.wrap", "a.txt", cause)
	if !errors.Is(err, os.ErrNotExist) || err.Error() != "lendo a.txt: file does not exist" {
		t.Errorf("%v", err)
	}
}

func TestMessagesByPrefix(t *testing.T) {
	catalog["web.test.a"] = m{"A", "Á", "Á"}
	catalog["web.test.b"] = m{"B", "", "B!"} // pt missing: English fills in
	catalog["cli.test.c"] = m{"C", "C", "C"}
	defer delete(catalog, "web.test.a")
	defer delete(catalog, "web.test.b")
	defer delete(catalog, "cli.test.c")
	got := Messages(PtBR, "web.test.")
	if len(got) != 2 || got["web.test.a"] != "Á" || got["web.test.b"] != "B" {
		t.Errorf("%v", got)
	}
}

func TestAddRejectsDuplicateKeys(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a duplicate key must panic at start-up, not silently overwrite")
		}
	}()
	add(map[string]m{"lang.unsupported": {"x", "x", "x"}})
}
