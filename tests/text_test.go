// text_test.go
package main

import "testing"

func TestSanitiseBasic(t *testing.T) {
    input := "abc { def } ghi"
    out := sanitise(input)
    // At minimum, ensure it doesn’t return empty.
    if out == "" {
        t.Fatalf("sanitise(%q) returned empty string", input)
    }
}

func TestLgrepBasic(t *testing.T) {
    input := "apple\nbanana\ncherry\n"
    // lgrep should return the first matching line containing "an"
    out := lgrep(input, "an")
    if out != "banana" {
        t.Fatalf("lgrep(%q, %q) expected %q, got %q", input, "an", "banana", out)
    }
}

