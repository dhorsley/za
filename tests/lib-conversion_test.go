package main

import (
	"testing"
)

func init() {
	buildStandardLib()
}

func TestHexEncodeString(t *testing.T) {
	got, err := stdlib["hex_encode"]("", 0, nil, "Hello")
	if err != nil {
		t.Fatalf("hex_encode(Hello) failed: %v", err)
	}
	if got.(string) != "48656c6c6f" {
		t.Errorf("hex_encode(Hello) = %q, want 48656c6c6f", got.(string))
	}

	// Empty string
	got, err = stdlib["hex_encode"]("", 0, nil, "")
	if err != nil {
		t.Fatalf("hex_encode() failed: %v", err)
	}
	if got.(string) != "" {
		t.Errorf("hex_encode() = %q, want empty", got.(string))
	}

	// Special bytes
	got, err = stdlib["hex_encode"]("", 0, nil, "\x00\x01\xff")
	if err != nil {
		t.Fatalf("hex_encode(special) failed: %v", err)
	}
	if got.(string) != "0001ff" {
		t.Errorf("hex_encode(special) = %q, want 0001ff", got.(string))
	}
}

func TestHexEncodeBytes(t *testing.T) {
	bytes := []uint8{72, 101, 108, 108, 111}
	got, err := stdlib["hex_encode"]("", 0, nil, bytes)
	if err != nil {
		t.Fatalf("hex_encode(bytes) failed: %v", err)
	}
	if got.(string) != "48656c6c6f" {
		t.Errorf("hex_encode(bytes) = %q, want 48656c6c6f", got.(string))
	}

	// Empty bytes
	empty := []uint8{}
	got, err = stdlib["hex_encode"]("", 0, nil, empty)
	if err != nil {
		t.Fatalf("hex_encode(empty) failed: %v", err)
	}
	if got.(string) != "" {
		t.Errorf("hex_encode(empty) = %q, want empty", got.(string))
	}
}

func TestHexDecode(t *testing.T) {
	got, err := stdlib["hex_decode"]("", 0, nil, "48656c6c6f")
	if err != nil {
		t.Fatalf("hex_decode(48656c6c6f) failed: %v", err)
	}
	b := got.([]uint8)
	if len(b) != 5 || b[0] != 72 || b[1] != 101 || b[2] != 108 || b[3] != 108 || b[4] != 111 {
		t.Errorf("hex_decode(48656c6c6f) = %v, want [72 101 108 108 111]", b)
	}

	// Empty string
	got, err = stdlib["hex_decode"]("", 0, nil, "")
	if err != nil {
		t.Fatalf("hex_decode() failed: %v", err)
	}
	b = got.([]uint8)
	if len(b) != 0 {
		t.Errorf("hex_decode() = %v, want empty", b)
	}

	// Uppercase
	got, err = stdlib["hex_decode"]("", 0, nil, "48656C6C6F")
	if err != nil {
		t.Fatalf("hex_decode(uppercase) failed: %v", err)
	}
	b = got.([]uint8)
	if len(b) != 5 || b[0] != 72 {
		t.Errorf("hex_decode(uppercase) = %v, want [72 ...]", b)
	}

	// Odd length should error
	_, err = stdlib["hex_decode"]("", 0, nil, "abc")
	if err == nil {
		t.Errorf("hex_decode(abc) should have failed with odd length")
	}

	// Invalid hex should error
	_, err = stdlib["hex_decode"]("", 0, nil, "zzzz")
	if err == nil {
		t.Errorf("hex_decode(zzzz) should have failed with invalid hex")
	}
}

func TestHexRoundTrip(t *testing.T) {
	original := "The quick brown fox jumps over the lazy dog!"
	encoded, err := stdlib["hex_encode"]("", 0, nil, original)
	if err != nil {
		t.Fatalf("hex_encode failed: %v", err)
	}

	decoded, err := stdlib["hex_decode"]("", 0, nil, encoded.(string))
	if err != nil {
		t.Fatalf("hex_decode failed: %v", err)
	}

	b := decoded.([]uint8)
	roundTrip := string(b)
	if roundTrip != original {
		t.Errorf("round-trip failed: %q != %q", roundTrip, original)
	}
}

func TestUrlEncode(t *testing.T) {
	got, err := stdlib["url_encode"]("", 0, nil, "hello world")
	if err != nil {
		t.Fatalf("url_encode failed: %v", err)
	}
	if got.(string) != "hello+world" {
		t.Errorf("url_encode(hello world) = %q, want hello+world", got.(string))
	}

	// No special chars
	got, err = stdlib["url_encode"]("", 0, nil, "hello")
	if err != nil {
		t.Fatalf("url_encode failed: %v", err)
	}
	if got.(string) != "hello" {
		t.Errorf("url_encode(hello) = %q, want hello", got.(string))
	}

	// Empty string
	got, err = stdlib["url_encode"]("", 0, nil, "")
	if err != nil {
		t.Fatalf("url_encode failed: %v", err)
	}
	if got.(string) != "" {
		t.Errorf("url_encode() = %q, want empty", got.(string))
	}
}

func TestUrlDecode(t *testing.T) {
	got, err := stdlib["url_decode"]("", 0, nil, "hello%20world")
	if err != nil {
		t.Fatalf("url_decode failed: %v", err)
	}
	if got.(string) != "hello world" {
		t.Errorf("url_decode(hello%%20world) = %q, want hello world", got.(string))
	}

	// Plus handling
	got, err = stdlib["url_decode"]("", 0, nil, "hello+world")
	if err != nil {
		t.Fatalf("url_decode failed: %v", err)
	}
	if got.(string) != "hello world" {
		t.Errorf("url_decode(hello+world) = %q, want hello world", got.(string))
	}

	// Special chars
	got, err = stdlib["url_decode"]("", 0, nil, "a%2Bb%3Dc%26d%3De")
	if err != nil {
		t.Fatalf("url_decode failed: %v", err)
	}
	if got.(string) != "a+b=c&d=e" {
		t.Errorf("url_decode(special) = %q, want a+b=c&d=e", got.(string))
	}

	// Empty string
	got, err = stdlib["url_decode"]("", 0, nil, "")
	if err != nil {
		t.Fatalf("url_decode failed: %v", err)
	}
	if got.(string) != "" {
		t.Errorf("url_decode() = %q, want empty", got.(string))
	}

	// Invalid encoding should error
	_, err = stdlib["url_decode"]("", 0, nil, "%zz")
	if err == nil {
		t.Errorf("url_decode(%%zz) should have failed")
	}
}

func TestUrlRoundTrip(t *testing.T) {
	originals := []string{
		"hello world",
		"a+b=c&d=e",
		"path/to/file.txt",
		"key=value&foo=bar",
	}

	for _, original := range originals {
		encoded, err := stdlib["url_encode"]("", 0, nil, original)
		if err != nil {
			t.Fatalf("url_encode(%q) failed: %v", original, err)
		}

		decoded, err := stdlib["url_decode"]("", 0, nil, encoded.(string))
		if err != nil {
			t.Fatalf("url_decode(%q) failed: %v", encoded.(string), err)
		}

		// Note: QueryEscape/QueryUnescape may not be perfectly round-trip for some chars
		if decoded.(string) != original {
			t.Errorf("round-trip failed for %q: got %q", original, decoded.(string))
		}
	}
}
