package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash/crc32"
	"os"
	"testing"
)

func init() {
	buildStandardLib()
}

func TestMd5sumFile(t *testing.T) {
	// Create temp file with known content
	content := []byte("The quick brown fox jumps over the lazy dog")
	tmpfile, err := os.CreateTemp("", "test-md5-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tmpfile.Close()

	// Calculate expected MD5
	h := md5.New()
	h.Write(content)
	expected := hex.EncodeToString(h.Sum(nil))

	// Test the function
	got, err := stdlib["md5sum_file"]("", 0, nil, tmpfile.Name())
	if err != nil {
		t.Fatalf("md5sum_file failed: %v", err)
	}
	if got.(string) != expected {
		t.Errorf("md5sum_file = %q, want %q", got.(string), expected)
	}
}

func TestSha1sumFile(t *testing.T) {
	content := []byte("The quick brown fox jumps over the lazy dog")
	tmpfile, err := os.CreateTemp("", "test-sha1-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tmpfile.Close()

	h := sha1.New()
	h.Write(content)
	expected := hex.EncodeToString(h.Sum(nil))

	got, err := stdlib["sha1sum_file"]("", 0, nil, tmpfile.Name())
	if err != nil {
		t.Fatalf("sha1sum_file failed: %v", err)
	}
	if got.(string) != expected {
		t.Errorf("sha1sum_file = %q, want %q", got.(string), expected)
	}
}

func TestSha256sumFile(t *testing.T) {
	content := []byte("The quick brown fox jumps over the lazy dog")
	tmpfile, err := os.CreateTemp("", "test-sha256-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tmpfile.Close()

	h := sha256.New()
	h.Write(content)
	expected := hex.EncodeToString(h.Sum(nil))

	got, err := stdlib["sha256sum_file"]("", 0, nil, tmpfile.Name())
	if err != nil {
		t.Fatalf("sha256sum_file failed: %v", err)
	}
	if got.(string) != expected {
		t.Errorf("sha256sum_file = %q, want %q", got.(string), expected)
	}
}

func TestSha512sumFile(t *testing.T) {
	content := []byte("The quick brown fox jumps over the lazy dog")
	tmpfile, err := os.CreateTemp("", "test-sha512-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tmpfile.Close()

	h := sha512.New()
	h.Write(content)
	expected := hex.EncodeToString(h.Sum(nil))

	got, err := stdlib["sha512sum_file"]("", 0, nil, tmpfile.Name())
	if err != nil {
		t.Fatalf("sha512sum_file failed: %v", err)
	}
	if got.(string) != expected {
		t.Errorf("sha512sum_file = %q, want %q", got.(string), expected)
	}
}

func TestCrc32File(t *testing.T) {
	content := []byte("The quick brown fox jumps over the lazy dog")
	tmpfile, err := os.CreateTemp("", "test-crc32-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tmpfile.Close()

	h := crc32.NewIEEE()
	h.Write(content)
	expected := hex.EncodeToString(h.Sum(nil))

	got, err := stdlib["crc32_file"]("", 0, nil, tmpfile.Name())
	if err != nil {
		t.Fatalf("crc32_file failed: %v", err)
	}
	if got.(string) != expected {
		t.Errorf("crc32_file = %q, want %q", got.(string), expected)
	}
}

func TestMd5sumBytes(t *testing.T) {
	content := []byte("The quick brown fox jumps over the lazy dog")
	
	h := md5.New()
	h.Write(content)
	expected := hex.EncodeToString(h.Sum(nil))

	got, err := stdlib["md5sum_bytes"]("", 0, nil, []uint8(content))
	if err != nil {
		t.Fatalf("md5sum_bytes failed: %v", err)
	}
	if got.(string) != expected {
		t.Errorf("md5sum_bytes = %q, want %q", got.(string), expected)
	}

	// Empty bytes
	got, err = stdlib["md5sum_bytes"]("", 0, nil, []uint8{})
	if err != nil {
		t.Fatalf("md5sum_bytes(empty) failed: %v", err)
	}
	// d41d8cd98f00b204e9800998ecf8427e is the MD5 of empty string
	if got.(string) != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("md5sum_bytes(empty) = %q, want d41d8cd98f00b204e9800998ecf8427e", got.(string))
	}
}

func TestSha256sumBytes(t *testing.T) {
	content := []byte("The quick brown fox jumps over the lazy dog")
	
	h := sha256.New()
	h.Write(content)
	expected := hex.EncodeToString(h.Sum(nil))

	got, err := stdlib["sha256sum_bytes"]("", 0, nil, []uint8(content))
	if err != nil {
		t.Fatalf("sha256sum_bytes failed: %v", err)
	}
	if got.(string) != expected {
		t.Errorf("sha256sum_bytes = %q, want %q", got.(string), expected)
	}
}

func TestFileChecksumMatchesStringChecksum(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog"
	
	// Write to temp file
	tmpfile, err := os.CreateTemp("", "test-match-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	tmpfile.Close()

	// Compare file and string checksums
	fileMd5, _ := stdlib["md5sum_file"]("", 0, nil, tmpfile.Name())
	stringMd5, _ := stdlib["md5sum"]("", 0, nil, content)
	if fileMd5.(string) != stringMd5.(string) {
		t.Errorf("md5sum_file != md5sum: %q != %q", fileMd5.(string), stringMd5.(string))
	}

	fileSha1, _ := stdlib["sha1sum_file"]("", 0, nil, tmpfile.Name())
	stringSha1, _ := stdlib["sha1sum"]("", 0, nil, content)
	if fileSha1.(string) != stringSha1.(string) {
		t.Errorf("sha1sum_file != sha1sum: %q != %q", fileSha1.(string), stringSha1.(string))
	}

	fileSha256, _ := stdlib["sha256sum_file"]("", 0, nil, tmpfile.Name())
	stringSha256, _ := stdlib["sha256sum"]("", 0, nil, content)
	if fileSha256.(string) != stringSha256.(string) {
		t.Errorf("sha256sum_file != sha256sum: %q != %q", fileSha256.(string), stringSha256.(string))
	}
}

func TestChecksumNonExistentFile(t *testing.T) {
	_, err := stdlib["md5sum_file"]("", 0, nil, "/nonexistent/path/to/file.txt")
	if err == nil {
		t.Errorf("md5sum_file on non-existent file should have failed")
	}

	_, err = stdlib["sha256sum_file"]("", 0, nil, "/nonexistent/path/to/file.txt")
	if err == nil {
		t.Errorf("sha256sum_file on non-existent file should have failed")
	}
}
