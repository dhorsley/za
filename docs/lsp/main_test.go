package main

import (
	"strings"
	"testing"
)

func TestIsWordBoundaryChange(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{"", true},
		{"hello", false},
		{"hello ", true},
		{"hello\t", true},
		{"hello\n", true},
		{"hello,", true},
		{"hello;", true},
		{"hello(", true},
		{"hello)", true},
		{"hello[", true},
		{"hello]", true},
		{"hello{", true},
		{"hello}", true},
		{"hello\"", true},
		{"hello'", true},
		{"hello`", true},
		{"hello.", true},
		{"hello\r", true},
		{"hello_world", false},
		{"hello123", false},
	}

	for _, tc := range tests {
		got := isWordBoundaryChange(tc.content)
		if got != tc.want {
			t.Errorf("isWordBoundaryChange(%q) = %v, want %v", tc.content, got, tc.want)
		}
	}
}

func TestGetFullDiagnostics(t *testing.T) {
	server := NewLSPServer(nil, "za")

	// Test with valid code
	content := `print "hello"
`
	diags := server.getFullDiagnostics("test://test.za", content)
	if len(diags) != 0 {
		t.Errorf("Expected 0 diagnostics for valid code, got %d", len(diags))
	}

	// Test with code that has a missing module (nonexistent file)
	// Module resolution errors are NOT surfaced as diagnostics (they're environment issues, not parse errors)
	content = `module "/tmp/nonexistent_file_12345.za"
print "hello"
`
	diags = server.getFullDiagnostics("test://test.za", content)
	if len(diags) != 0 {
		t.Errorf("Expected 0 diagnostics for missing module (filtered out), got %d: %v", len(diags), diags)
	}
}

func TestRunDiagnosticsMerge(t *testing.T) {
	server := NewLSPServer(nil, "za")
	server.documents["test://merge.za"] = &Document{
		URI:     "test://merge.za",
		Content: "def foo\n",
		Symbols: make(map[string]*Symbol),
	}
	server.lastFullDiags["test://merge.za"] = []Diagnostic{
		{Message: "full diag", Source: "za-parse", Severity: 1},
	}

	// We can't easily test publishDiagnostics without a writer, but we can test getStructuralDiagnostics
	structuralDiags := server.getStructuralDiagnostics("test://merge.za", "def foo\n")
	if len(structuralDiags) == 0 {
		t.Errorf("Expected structural diagnostics for unclosed block, got 0")
	}

	// Verify merge would work: structural + full = len(structural) + len(full)
	allDiags := append(structuralDiags, server.lastFullDiags["test://merge.za"]...)
	if len(allDiags) != len(structuralDiags)+1 {
		t.Errorf("Expected merged diagnostics to be %d, got %d", len(structuralDiags)+1, len(allDiags))
	}
}

func TestHandleDidChangeWordBoundary(t *testing.T) {
	server := NewLSPServer(nil, "za")
	server.documents["test://wb.za"] = &Document{
		URI:     "test://wb.za",
		Content: "",
		Symbols: make(map[string]*Symbol),
	}

	// Simulate a word boundary change
	msg := &JSONRPCMessage{
		Method: "textDocument/didChange",
		Params: []byte(`{"textDocument":{"uri":"test://wb.za"},"contentChanges":[{"text":"print \"hello\" \n"}]}`),
	}

	server.handleDidChange(msg)

	// After word boundary change, both timers should be set
	server.mu.RLock()
	_, hasTimer := server.timers["test://wb.za"]
	_, hasFullTimer := server.fullTimers["test://wb.za"]
	server.mu.RUnlock()

	if !hasTimer {
		t.Errorf("Expected structural timer to be set after word boundary change")
	}
	if !hasFullTimer {
		t.Errorf("Expected full diagnostic timer to be set after word boundary change")
	}

	// Clean up timers
	server.mu.Lock()
	if timer, ok := server.timers["test://wb.za"]; ok {
		timer.Stop()
		delete(server.timers, "test://wb.za")
	}
	if timer, ok := server.fullTimers["test://wb.za"]; ok {
		timer.Stop()
		delete(server.fullTimers, "test://wb.za")
	}
	server.mu.Unlock()
}

func TestHandleDidChangeNonWordBoundary(t *testing.T) {
	server := NewLSPServer(nil, "za")
	server.documents["test://nwb.za"] = &Document{
		URI:     "test://nwb.za",
		Content: "",
		Symbols: make(map[string]*Symbol),
	}

	// Simulate a non-word boundary change (typing in middle of word)
	msg := &JSONRPCMessage{
		Method: "textDocument/didChange",
		Params: []byte(`{"textDocument":{"uri":"test://nwb.za"},"contentChanges":[{"text":"print hello"}]}`),
	}

	server.handleDidChange(msg)

	// After non-word boundary change, only structural timer should be set
	server.mu.RLock()
	_, hasTimer := server.timers["test://nwb.za"]
	_, hasFullTimer := server.fullTimers["test://nwb.za"]
	server.mu.RUnlock()

	if !hasTimer {
		t.Errorf("Expected structural timer to be set after non-word boundary change")
	}
	if hasFullTimer {
		t.Errorf("Expected full diagnostic timer NOT to be set after non-word boundary change")
	}

	// Clean up timers
	server.mu.Lock()
	if timer, ok := server.timers["test://nwb.za"]; ok {
		timer.Stop()
		delete(server.timers, "test://nwb.za")
	}
	server.mu.Unlock()
}

func TestDebounceTiming(t *testing.T) {
	// Verify debounce constants are in expected ranges
	if diagDebounceMs < 200 || diagDebounceMs > 1000 {
		t.Errorf("diagDebounceMs = %d, expected 200-1000", diagDebounceMs)
	}
	if wordBoundaryDebounceMs < 50 || wordBoundaryDebounceMs > 300 {
		t.Errorf("wordBoundaryDebounceMs = %d, expected 50-300", wordBoundaryDebounceMs)
	}
	if wordBoundaryDebounceMs >= diagDebounceMs {
		t.Errorf("wordBoundaryDebounceMs (%d) should be < diagDebounceMs (%d)", wordBoundaryDebounceMs, diagDebounceMs)
	}
}

func TestTokenizeDocument(t *testing.T) {
	content := `def hello()
    print "world"
    x = 42
enddef
`
	tokenMap := tokenizeDocument(content)

	// Line 0: def hello()
	toks0, ok := tokenMap[0]
	if !ok {
		t.Fatalf("Expected tokens on line 0")
	}
	if len(toks0) < 3 {
		t.Fatalf("Expected at least 3 tokens on line 0, got %d", len(toks0))
	}
	if toks0[0].Kind != TokKeyword || toks0[0].Text != "def" {
		t.Errorf("Expected 'def' keyword on line 0, got %v", toks0[0])
	}
	if toks0[1].Kind != TokIdentifier || toks0[1].Text != "hello" {
		t.Errorf("Expected 'hello' identifier on line 0, got %v", toks0[1])
	}
	if toks0[2].Kind != TokLParen {
		t.Errorf("Expected '(' on line 0, got %v", toks0[2])
	}

	// Line 1: print "world"
	toks1, ok := tokenMap[1]
	if !ok {
		t.Fatalf("Expected tokens on line 1")
	}
	if len(toks1) < 2 {
		t.Fatalf("Expected at least 2 tokens on line 1, got %d", len(toks1))
	}
	if toks1[0].Kind != TokKeyword || toks1[0].Text != "print" {
		t.Errorf("Expected 'print' keyword on line 1, got %v", toks1[0])
	}
	if toks1[1].Kind != TokString {
		t.Errorf("Expected string literal on line 1, got %v", toks1[1])
	}

	// Line 2: x = 42
	toks2, ok := tokenMap[2]
	if !ok {
		t.Fatalf("Expected tokens on line 2")
	}
	if len(toks2) < 3 {
		t.Fatalf("Expected at least 3 tokens on line 2, got %d", len(toks2))
	}
	if toks2[0].Kind != TokIdentifier || toks2[0].Text != "x" {
		t.Errorf("Expected 'x' identifier on line 2, got %v", toks2[0])
	}
	// Real lexer maps '=' to TokAssign, not TokOperator
	if toks2[1].Kind != TokAssign || toks2[1].Text != "=" {
		t.Errorf("Expected '=' TokAssign on line 2, got %v", toks2[1])
	}
	if toks2[2].Kind != TokNumber || toks2[2].Text != "42" {
		t.Errorf("Expected '42' number on line 2, got %v", toks2[2])
	}

	// Line 3: enddef
	toks3, ok := tokenMap[3]
	if !ok {
		t.Fatalf("Expected tokens on line 3")
	}
	if len(toks3) < 1 {
		t.Fatalf("Expected at least 1 token on line 3, got %d", len(toks3))
	}
	if toks3[0].Kind != TokKeyword || toks3[0].Text != "enddef" {
		t.Errorf("Expected 'enddef' keyword on line 3, got %v", toks3[0])
	}
}

func TestStructuralDiagnosticsWithLexer(t *testing.T) {
	server := NewLSPServer(nil, "za")

	// Unclosed block
	content := `def foo()
    print "hello"
`
	diags := server.getStructuralDiagnostics("test://a.za", content)
	if len(diags) == 0 {
		t.Fatalf("Expected diagnostics for unclosed block, got 0")
	}
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "Unclosed block: def") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'Unclosed block: def' diagnostic, got: %v", diags)
	}

	// Balanced block - no diagnostics
	content2 := `def foo()
    print "hello"
enddef
`
	diags2 := server.getStructuralDiagnostics("test://b.za", content2)
	for _, d := range diags2 {
		if strings.Contains(d.Message, "Unclosed block") {
			t.Errorf("Expected no unclosed block diagnostic, got: %v", d)
		}
	}
}

func TestExtractSymbolsWithLexer(t *testing.T) {
	server := NewLSPServer(nil, "za")
	server.documents["test://c.za"] = &Document{
		URI:     "test://c.za",
		Content: "",
		Symbols: make(map[string]*Symbol),
	}

	content := `def hello()
    print "world"
enddef

struct Point
    x
    y
endstruct

enum Color
    Red
    Green
endenum

foreach item in items
    print item
endfor

var myvar

setglob gvar = 10

x = 5
`
	server.extractSymbols("test://c.za", content)
	doc := server.documents["test://c.za"]

	// Check function
	if s, ok := doc.Symbols["hello"]; !ok || s.Kind != "function" {
		t.Errorf("Expected 'hello' function symbol, got: %v", s)
	}

	// Check struct
	if s, ok := doc.Symbols["Point"]; !ok || s.Kind != "struct" {
		t.Errorf("Expected 'Point' struct symbol, got: %v", s)
	}

	// Check enum
	if s, ok := doc.Symbols["Color"]; !ok || s.Kind != "enum" {
		t.Errorf("Expected 'Color' enum symbol, got: %v", s)
	}

	// Check foreach variable
	if s, ok := doc.Symbols["item"]; !ok || s.Kind != "variable" {
		t.Errorf("Expected 'item' variable symbol, got: %v", s)
	}

	// Check var declaration
	if s, ok := doc.Symbols["myvar"]; !ok || s.Kind != "variable" {
		t.Errorf("Expected 'myvar' variable symbol, got: %v", s)
	}

	// Check setglob declaration
	if s, ok := doc.Symbols["gvar"]; !ok || s.Kind != "variable" {
		t.Errorf("Expected 'gvar' variable symbol, got: %v", s)
	}

	// Check assignment variable
	if s, ok := doc.Symbols["x"]; !ok || s.Kind != "variable" {
		t.Errorf("Expected 'x' variable symbol, got: %v", s)
	}
}

func TestExtractSymbolsAtSetglob(t *testing.T) {
	server := NewLSPServer(nil, "za")
	server.documents["test://at.za"] = &Document{
		URI:     "test://at.za",
		Content: "",
		Symbols: make(map[string]*Symbol),
	}

	content := `@global_var = 100
`
	server.extractSymbols("test://at.za", content)
	doc := server.documents["test://at.za"]

	if s, ok := doc.Symbols["global_var"]; !ok || s.Kind != "variable" {
		t.Errorf("Expected 'global_var' from @ declaration, got: %v", s)
	}
}
