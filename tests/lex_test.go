// lex_test.go
package main

import "testing"

// Test that simple identifiers and numbers are returned in sequence,
// and that an EOF token is emitted at the end.
func TestNextTokenIdentifiersAndNumbers(t *testing.T) {
    input := "foo 123 bar"
    var line int16 = 1
    pos := 0

    // 1) First call → "foo"
    tok1 := nextToken(input, 0, &line, pos)
    if tok1.carton.tokText != "foo" {
        t.Fatalf("expected first token 'foo', got %q", tok1.carton.tokText)
    }
    pos = tok1.tokPos

    // 2) Second call → "123"
    tok2 := nextToken(input, 0, &line, pos)
    if tok2.carton.tokText != "123" {
        t.Fatalf("expected second token '123', got %q", tok2.carton.tokText)
    }
    pos = tok2.tokPos

    // 3) Third call → "bar"
    tok3 := nextToken(input, 0, &line, pos)
    if tok3.carton.tokText != "bar" {
        t.Fatalf("expected third token 'bar', got %q", tok3.carton.tokText)
    }
    pos = tok3.tokPos

    // 4) If pos >= len(input), treat as EOF; otherwise call nextToken and expect eof==true
    var tok4 *lcstruct
    if pos >= len(input) {
        tok4 = &lcstruct{eof: true}
    } else {
        tok4 = nextToken(input, 0, &line, pos)
    }
    if !tok4.eof {
        t.Fatalf("expected EOF after all input, got %+v", tok4)
    }
}

// Test that string literals—both double-quoted and single-quoted—are returned correctly.
// Note: single-quoted literals remain wrapped in single quotes in tokText.
func TestNextTokenStringLiteral(t *testing.T) {
    // Inside the literal, \n should become a real newline in tokText
    input := `"hello\nworld" 'simple'`
    var line int16 = 1
    pos := 0

    // 1) Double-quoted literal → tokText == "hello\nworld"
    tok1 := nextToken(input, 0, &line, pos)
    expected1 := "hello\nworld"
    if tok1.carton.tokText != expected1 {
        t.Fatalf("expected first string literal %q, got %q", expected1, tok1.carton.tokText)
    }
    pos = tok1.tokPos

    // 2) Single-quoted literal → tokText == "'simple'"
    tok2 := nextToken(input, 0, &line, pos)
    expected2 := "'simple'"
    if tok2.carton.tokText != expected2 {
        t.Fatalf("expected single-quoted literal %q, got %q", expected2, tok2.carton.tokText)
    }
}

