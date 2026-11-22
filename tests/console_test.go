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

func TestCmdSepByteAcceptsInt(t *testing.T) {
    original, had := gvget("@cmdsep")
    defer func() {
        if had {
            gvset("@cmdsep", original)
        }
    }()

    gvset("@cmdsep", int(0x1e))
    if got := cmdSepByte(); got != 0x1e {
        t.Fatalf("cmdSepByte() with int separator = %d, want %d", got, 0x1e)
    }

    gvset("@cmdsep", byte(0x1d))
    if got := cmdSepByte(); got != 0x1d {
        t.Fatalf("cmdSepByte() with byte separator = %d, want %d", got, 0x1d)
    }

    gvset("@cmdsep", "30")
    if got := cmdSepByte(); got != 0x1e {
        t.Fatalf("cmdSepByte() with string separator = %d, want %d", got, 0x1e)
    }
}

func TestCmdSepByteFallback(t *testing.T) {
    original, had := gvget("@cmdsep")
    defer func() {
        if had {
            gvset("@cmdsep", original)
        }
    }()

    for _, value := range []any{int(-1), int(256), uint64(512), "bad", nil} {
        gvset("@cmdsep", value)
        if got := cmdSepByte(); got != defaultCmdSep {
            t.Fatalf("cmdSepByte() with %#v = %d, want fallback %d", value, got, defaultCmdSep)
        }
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

