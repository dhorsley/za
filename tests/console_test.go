// console_test.go
package main

import (
    "fmt"
    "testing"
)

func TestStrip_UnchangedPlainText(t *testing.T) {
    input := "  hello world  "
    out := Strip(input)
    if out != input {
        t.Fatalf("Strip(%q) should return unchanged text, got %q", input, out)
    }
}

func TestStrip_RemovesANSI_Escapes(t *testing.T) {
    ansiWrapped := "\u001b[31mfoo\u001b[0m" // ANSI red-on-black around "foo"
    out := Strip(ansiWrapped)
    if out != "foo" {
        t.Fatalf("Strip(%q) expected %q, got %q", ansiWrapped, "foo", out)
    }
}

func TestStripCC_RemovesFairydustTags(t *testing.T) {
    // 1) Enable ANSI mode so strip/replacer logic runs
    ansiMode = true

    // 2) Populate fairydust via setup function
    setupAnsiPalette()

    // 3) Pick any valid key from fairydust
    var someKey string
    for k := range fairydust {
        someKey = k
        break
    }
    if someKey == "" {
        t.Fatal("fairydust is empty even after setupAnsiPalette()")
    }

    // 4) Build a test string using that key
    opening := fmt.Sprintf("[#%s]", someKey)
    closing := "[#-]"
    input := opening + "hello" + closing

    // 5) StripCC should remove both opening "[#key]" and closing "[#-]"
    out := StripCC(input)
    if out != "hello" {
        t.Fatalf("StripCC(%q) expected %q, got %q", input, "hello", out)
    }
}

