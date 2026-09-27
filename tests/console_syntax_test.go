package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFuzzyTermMatch(t *testing.T) {
	cases := []struct {
		term, target string
		want         bool
	}{
		{"git status", "git status", true},   // exact
		{"git st", "git status", true},       // substring
		{"gts", "git status", true},          // subsequence
		{"git", "git commit", true},          // substring
		{"zq", "git status", false},          // not a subsequence
		{"", "anything", true},               // empty term
		{"status", "git log", false},         // no match
	}
	for _, c := range cases {
		if got := fuzzyTermMatch(c.term, c.target); got != c.want {
			t.Errorf("fuzzyTermMatch(%q,%q)=%v want %v", c.term, c.target, got, c.want)
		}
	}
}

func TestAcceptSuggestionWord(t *testing.T) {
	// candidate with trailing-space typing
	s := []rune("git ")
	cand := "git status"
	ns, npos, ok := acceptSuggestionWord(s, cand)
	if !ok || string(ns) != "git status" || npos != len("git status") {
		t.Fatalf("word accept = (%q,%d,%v)", string(ns), npos, ok)
	}
	// candidate typed without trailing space -> space inserted
	s = []rune("git")
	ns, npos, ok = acceptSuggestionWord(s, "git status")
	if !ok || string(ns) != "git status" {
		t.Fatalf("word accept no-space = (%q,%d,%v)", string(ns), npos, ok)
	}
	// function-name suggestion (no space in tail: direct continuation)
	s = []rune("for")
	ns, _, ok = acceptSuggestionWord(s, "format(")
	if !ok || string(ns) != "format(" {
		t.Fatalf("word accept fn = (%q, %v)", string(ns), ok)
	}
	// path-continuation suggestion (tail extends the final token, no space)
	s = []rune("ls doc")
	ns, _, ok = acceptSuggestionWord(s, "ls docs/")
	if !ok || string(ns) != "ls docs/" {
		t.Fatalf("word accept path = (%q, %v)", string(ns), ok)
	}
	// already identical -> false
	if _, _, ok := acceptSuggestionWord([]rune("same"), "same"); ok {
		t.Fatal("expected false for identical cand")
	}
}

func TestColourSyntax(t *testing.T) {
	ansiMode = true // main() sets this; ensure the palette emits real codes
	out := colourSyntax("if x == 42  # comment")
	if !hasSub(out, "\033[1m\033[93m") { // bold+yellow keyword
		t.Fatalf("keyword 'if' not highlighted: %q", out)
	}
	if !hasSub(out, "\033[94m42") { // blue number (palette [#1])
		t.Fatalf("number not highlighted: %q", out)
	}
	if !hasSub(out, "\033[2m") { // dim comment
		t.Fatalf("comment not dimmed: %q", out)
	}
	// strings
	s := colourSyntax(`a = "hello"`)
	if !hasSub(s, "\033[91m") { // string colour [#2] = red
		t.Fatalf("string not coloured: %q", s)
	}
	// a known stdlib function name (not a keyword) gets highlighted
	slhelp["format"] = LibHelp{in: "string, args", action: "format a string"}
	f := colourSyntax("format(x)")
	if !hasSub(f, "\033[96mformat") { // function colour [#5] = cyan
		t.Fatalf("function name not highlighted: %q", f)
	}
	// passthrough parity: visible text length equals input length
	if len(stripANSI(out)) != len("if x == 42  # comment") {
		t.Fatalf("visible length mismatch: %q", out)
	}
}

func TestPathSuggestion(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.MkdirAll(filepath.Join(dir, "proj"), 0o755)
	os.WriteFile(filepath.Join(dir, "sub", "main.za"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, ".hidden_file"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "a+b.txt"), []byte(""), 0o644)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	os.Chdir(dir)
	defer os.Chdir(old)

	// trailing slash on a unique dir completed relative to the cwd
	if got := pathSuggestion("cd ./su"); got != "cd ./sub/" {
		t.Fatalf("pathSuggestion relative = %q", got)
	}
	// no slash: completion against the current directory
	if got := pathSuggestion("cd su"); got != "cd sub/" {
		t.Fatalf("pathSuggestion no-slash = %q", got)
	}
	// directories are preferred and get the trailing slash
	if got := pathSuggestion("cd pro"); got != "cd proj/" {
		t.Fatalf("pathSuggestion dir-pref = %q", got)
	}
	// a dot is a literal dot, not regex "any character": '.h' must not
	// suggest files that don't start with a dot, and must suggest the
	// hidden file
	if got := pathSuggestion("ls .h"); got != "ls .hidden_file" {
		t.Fatalf("pathSuggestion dotfile = %q", got)
	}
	// regex metacharacters in a filename are matched literally
	if got := pathSuggestion("cat a+b"); got != "cat a+b.txt" {
		t.Fatalf("pathSuggestion metachar = %q", got)
	}
	// no matching entry -> ""
	if got := pathSuggestion("println"); got != "" {
		t.Fatalf("pathSuggestion non-path = %q", got)
	}
	if got := pathSuggestion("cat zzz"); got != "" {
		t.Fatalf("pathSuggestion no-match = %q", got)
	}
}

func TestWordStep(t *testing.T) {
	s := []rune("bb aa cc") // idx: b0 b1 ' '2 a3 a4 ' '5 c6 c7
	// from the end, forward is clamped
	if got := wordStep(s, len(s), +1); got != len(s) {
		t.Fatalf("forward at end = %d", got)
	}
	// backward one word from the end lands at the start of 'cc'
	if got := wordStep(s, len(s), -1); got != 6 {
		t.Fatalf("backward one word = %d (want 6)", got)
	}
	// backward again lands at the start of 'aa'
	if got := wordStep(s, 6, -1); got != 3 {
		t.Fatalf("backward two words = %d (want 3)", got)
	}
	// forward one word moves just past the word run
	if got := wordStep(s, 3, +1); got != 5 {
		t.Fatalf("forward one word = %d (want 5)", got)
	}
	// clamped at the start
	if got := wordStep(s, 0, -1); got != 0 {
		t.Fatalf("backward at start = %d", got)
	}
	// spaces-only tail
	if got := wordStep([]rune("a  "), 3, -1); got != 0 {
		t.Fatalf("backward over spaces = %d (want 0)", got)
	}
}

func stripANSI(s string) string {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++
			continue
		}
		out = append(out, rune(s[i]))
		i++
	}
	return string(out)
}

func TestDecodeEscKey(t *testing.T) {
	// esc f / kitty CSI-u alt-f -> accept
	if r, ok := decodeEscKey([]byte{0x1B, 0x66}); !ok || !r.accept {
		t.Fatal("ESC f should decode as alt-f accept")
	}
	if r, ok := decodeEscKey([]byte{0x1B, 0x5B, '1', '0', '2', ';', '3', 'u'}); !ok || !r.accept {
		t.Fatal("ESC [102;3u should decode as alt-f accept")
	}
	// xterm modified arrows
	if r, ok := decodeEscKey([]byte{0x1B, 0x5B, '1', ';', '3', 'C'}); !ok || r.dir != +1 || !r.word {
		t.Fatal("ESC [1;3C should decode alt-right word move")
	}
	if r, ok := decodeEscKey([]byte{0x1B, 0x5B, '1', ';', '5', 'D'}); !ok || r.dir != -1 || !r.word {
		t.Fatal("ESC [1;5D should decode ctrl-left word move")
	}
	if r, ok := decodeEscKey([]byte{0x1B, 0x5B, '1', ';', '2', 'C'}); !ok || r.dir != +1 || !r.word {
		t.Fatal("ESC [1;2C should decode shift-right word move")
	}
	// CSI-u arrows (kitty/ghostty)
	if r, ok := decodeEscKey([]byte{0x1B, 0x5B, '2', ';', '3', 'u'}); !ok || r.dir != +1 || !r.word {
		t.Fatal("ESC [2;3u should decode alt-right word move")
	}
	if r, ok := decodeEscKey([]byte{0x1B, 0x5B, '1', ';', '2', 'u'}); !ok || r.dir != -1 || !r.word {
		t.Fatal("ESC [1;2u should decode shift-left word move")
	}
	// SS3 plain arrows
	if r, ok := decodeEscKey([]byte{0x1B, 0x4F, 'C'}); !ok || r.dir != +1 || r.word {
		t.Fatal("ESC O C should decode plain right")
	}
	if r, ok := decodeEscKey([]byte{0x1B, 0x4F, 'D'}); !ok || r.dir != -1 || r.word {
		t.Fatal("ESC O D should decode plain left")
	}
	// plain CSI arrows decode as char moves (word=false)
	if r, ok := decodeEscKey([]byte{0x1B, 0x5B, 'C'}); !ok || r.dir != +1 || r.word {
		t.Fatal("ESC [C should decode plain right char move")
	}
	// unknown sequence is not a recognised key
	if _, ok := decodeEscKey([]byte{0x1B, 0x5B, '7', '~'}); ok {
		t.Fatal("ESC [7~ should not decode")
	}
	if _, ok := decodeEscKey([]byte{'x'}); ok {
		t.Fatal("plain text should not decode")
	}
}

func TestCompleteEscEnd(t *testing.T) {
	cases := []struct {
		b    []byte
		want int
	}{
		{[]byte{0x1B, 'f'}, 2},
		{[]byte{0x1B, 0x5B, 'C'}, 3},
		{[]byte{0x1B, 0x5B, '1', ';', '3', 'C'}, 6},
		{[]byte{0x1B, 0x5B, '1', '0', '2', ';', '3', 'u'}, 8},
		{[]byte{0x1B, 0x4F, 'C'}, 3},
		{[]byte{0x1B}, -1},
		{[]byte{0x1B, 0x5B, '1', ';'}, -1},
	}
	for _, c := range cases {
		if got := completeEscEnd(c.b); got != c.want {
			t.Fatalf("completeEscEnd(%v) = %d, want %d", c.b, got, c.want)
		}
	}
}

func TestSyntaxColoursStdlib(t *testing.T) {
	resetSyntaxColours()

	m := currentSyntaxColourMap()
	if m["keyword"] != "[#bold][#6]" {
		t.Fatalf("default keyword colour = %v", m["keyword"])
	}
	// set a subset; returns the previous effective map
	prev, err := setSyntaxColours(map[string]any{"error": "[#2][#b7]", "function": ""})
	if err != nil {
		t.Fatal(err)
	}
	if prev["error"] != "[#2]" {
		t.Fatalf("previous error colour = %v", prev["error"])
	}
	cur := currentSyntaxColourMap()
	if cur["error"] != "[#2][#b7]" || cur["function"] != "" {
		t.Fatalf("after set: error=%v function=%v", cur["error"], cur["function"])
	}
	// unknown class and bad markup are rejected
	if _, err := setSyntaxColours(map[string]any{"bogus": "[#9]"}); err == nil {
		t.Fatal("expected unknown-class error")
	}
	if _, err := setSyntaxColours(map[string]any{"error": "red"}); err == nil {
		t.Fatal("expected invalid-markup error")
	}
	// reset restores defaults
	resetSyntaxColours()
	if cur := currentSyntaxColourMap(); cur["function"] != "[#5]" {
		t.Fatalf("after reset function = %v", cur["function"])
	}
}

func TestColourSyntaxCommandPositions(t *testing.T) {
	ansiMode = true // ensure real ANSI codes are emitted
	resetSyntaxColours()
	if fairyReplacer == nil {
		setupAnsiPalette()
	}

	// isolate PATH and use distinctive word names so the process-wide lookup
	// cache cannot leak results from the real environment
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "zzcmd1"), []byte("#!/bin/sh\n"), 0o755)
	oldpath := os.Getenv("PATH")
	os.Setenv("PATH", dir)
	defer os.Setenv("PATH", oldpath)
	shellCmdCache.Delete("zzcmd1")
	shellCmdCache.Delete("zzmiss1")

	// give every class a distinctive colour for unambiguous assertions
	setSyntaxColours(map[string]any{
		"shellcommand": "[#7]",
		"error":        "[#6]",
		"operator":     "[#3]",
		"variable":     "[#4]",
	})

	// a valid shell command at command position gets the shellcommand class
	out := colourSyntax("zzcmd1 -x")
	if !hasSub(out, "\033[97mzzcmd1") {
		t.Fatalf("shell command not coloured: %q", out)
	}
	// an unknown word at command position gets the error class
	out = colourSyntax("zzmiss1 hello")
	if !hasSub(out, "\033[93mzzmiss1") {
		t.Fatalf("unknown command not error-coloured: %q", out)
	}
	// gating: after a confirmed shell command, `|` arms the next command
	// position and the separator is operator-coloured
	out = colourSyntax("zzcmd1 | zzmiss1")
	if !hasSub(out, "\033[93mzzmiss1") || !hasSub(out, "\033[95m|") {
		t.Fatalf("pipeline command not checked: %q", out)
	}
	// non-cascade: an unknown FIRST word does not validate later words
	out = colourSyntax("zzmiss1 | zzcmd1")
	if hasSub(out, "\033[97mzzcmd1") {
		t.Fatalf("shell check cascaded after unknown first word: %q", out)
	}
	// assignment target/values are not flagged as shell errors
	out = colourSyntax("m = zzmiss1")
	if hasSub(out, "\033[93m") {
		t.Fatalf("assignment line flagged as error: %q", out)
	}
	// ${...} command substitution validates its inner command
	if out := colourSyntax("a ${zzmiss1 arg}"); !hasSub(out, "\033[93mzzmiss1") {
		t.Fatalf("cmdsub unknown not error-coloured: %q", out)
	}
	if out := colourSyntax("a ${zzcmd1 arg}"); !hasSub(out, "\033[97mzzcmd1") {
		t.Fatalf("cmdsub valid command not coloured: %q", out)
	}
	// a za keyword statement stays shell-free: separators and later words are
	// not command-checked
	if out := colourSyntax("if x == 1; zzcmd1; endif"); hasSub(out, "\033[97mzzcmd1") {
		t.Fatalf("shell check leaked into za statement: %q", out)
	}

	// the renderer must never drop or shift characters: every visible char in
	// the input appears in the same order in the output
	for _, tc := range []string{"${ xyz }", "${ }", "${ xyz  more }", "${git status}", "a | b", "ls | grep"} {
		if got := stripANSI(colourSyntax(tc)); got != tc {
			t.Fatalf("colourSyntax %q lost characters: %q", tc, got)
		}
	}

	resetSyntaxColours()
	os.Setenv("PATH", oldpath)
}

func hasSub(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
			if s[i] == 0x1b {
				// skip anything that looks like an ANSI escape so partial
				// sequences in expectations still match cleanly
				for i < len(s) && s[i] != 'm' {
					i++
				}
			}
		}
		return false
	})()
}
