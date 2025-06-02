// phraser_test.go
package main

import "testing"

func TestPhraseParseSimple(t *testing.T) {
    // 1) Map "test" → 0 in fnlookup
    fnlookup.lmset("test", 0)

    // 2) Clear any existing phrases at index 0
    functionspaces[0] = []Phrase{}

    // 3) Parse "foo;" starting at 0; ignore eof return
    bad, _ := phraseParse("test", "foo;", 0)
    if bad {
        t.Fatalf("phraseParse returned badword=true unexpectedly")
    }

    // 4) After parsing, functionspaces[0] should contain exactly one Phrase
    if len(functionspaces[0]) != 1 {
        t.Fatalf("expected 1 phrase in functionspaces[0], got %d", len(functionspaces[0]))
    }

    ph := functionspaces[0][0]
    // That phrase should hold exactly one Token ("foo")
    if len(ph.Tokens) != 1 {
        t.Fatalf("expected phrase.Tokens length=1, got %d", len(ph.Tokens))
    }
    if ph.Tokens[0].tokText != "foo" {
        t.Fatalf("expected token text 'foo', got %q", ph.Tokens[0].tokText)
    }
}

