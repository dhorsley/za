//go:build linux || freebsd || openbsd || netbsd || dragonfly

package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// buildTestZip writes a zip containing the supplied entries and returns its path.
func buildTestZip(t *testing.T, entries map[string]string) string {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	// stable order keeps failures reproducible
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}

	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %q: %v", name, err)
		}
		if _, err := w.Write([]byte(entries[name])); err != nil {
			t.Fatalf("zip write %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	path := filepath.Join(t.TempDir(), "test.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	return path
}

func TestZipExtractRejectsTraversal(t *testing.T) {
	// Each entry escapes destDir once filepath.Join has cleaned it.
	escapes := map[string]string{
		"../escape_one.txt":  "one",
		"../../escape_two":   "two",
		"a/../../escape_thr": "three",
	}

	for name := range escapes {
		zpath := buildTestZip(t, escapes)

		root := t.TempDir()
		dest := filepath.Join(root, "dest")
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatalf("mkdir dest: %v", err)
		}

		zr, err := zip.OpenReader(zpath)
		if err != nil {
			t.Fatalf("open zip: %v", err)
		}
		defer zr.Close()

		for _, f := range zr.File {
			if err := extractFileFromZip(f, dest); err == nil {
				t.Errorf("extractFileFromZip(%q) succeeded, want traversal error", name)
			}
		}

		// Nothing may exist above dest.
		for _, victim := range []string{
			filepath.Join(root, "escape_one.txt"),
			filepath.Join(root, "escape_two"),
			filepath.Join(root, "escape_thr"),
		} {
			if _, err := os.Stat(victim); err == nil {
				t.Errorf("traversal wrote %q outside destination", victim)
			}
		}
	}
}

func TestZipExtractAllowsLegitimateEntries(t *testing.T) {
	entries := map[string]string{
		"top.txt":            "top",
		"sub/nested.txt":     "nested",
		"./dot.txt":          "dot",
		"sub/deep/deep2.txt": "deep",
	}

	zpath := buildTestZip(t, entries)
	dest := filepath.Join(t.TempDir(), "dest")

	zr, err := zip.OpenReader(zpath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if err := extractFileFromZip(f, dest); err != nil {
			t.Fatalf("extractFileFromZip(%q) unexpected error: %v", f.Name, err)
		}
	}

	// ./dot.txt must land as dot.txt, not literally "./dot.txt"
	for _, rel := range []string{"top.txt", "dot.txt", "sub/nested.txt", "sub/deep/deep2.txt"} {
		p := filepath.Join(dest, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected extracted file %q: %v", rel, err)
		}
	}
}

func TestZipExtractAbsoluteEntryStaysInside(t *testing.T) {
	// An absolute-looking entry name must be contained, not written to /etc.
	zpath := buildTestZip(t, map[string]string{"/etc/passwd": "nope"})
	root := t.TempDir()
	dest := filepath.Join(root, "dest")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	zr, err := zip.OpenReader(zpath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if err := extractFileFromZip(f, dest); err != nil {
			t.Fatalf("extractFileFromZip(%q) unexpected error: %v", f.Name, err)
		}
	}

	if _, err := os.Stat(filepath.Join(dest, "etc", "passwd")); err != nil {
		t.Errorf("absolute entry not contained under dest: %v", err)
	}
}

func TestZipSafeJoin(t *testing.T) {
	cases := []struct {
		dest    string
		name    string
		wantErr bool
	}{
		{"/tmp/dest", "file.txt", false},
		{"/tmp/dest", "sub/file.txt", false},
		{"/tmp/dest", "./file.txt", false},
		{"/tmp/dest", "sub/../file.txt", false},
		{"/tmp/dest", "/abs/file.txt", false}, // cleaned to dest/abs/file.txt
		{"/tmp/dest", "../file.txt", true},
		{"/tmp/dest", "../../file.txt", true},
		{"/tmp/dest", "sub/../../file.txt", true},
		{"/tmp/dest", "../dest-sibling/file.txt", true}, // prefix lookalike
	}

	for _, c := range cases {
		got, err := zipSafeJoin(c.dest, c.name)
		if c.wantErr {
			if err == nil {
				t.Errorf("zipSafeJoin(%q, %q) = %q, want error", c.dest, c.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("zipSafeJoin(%q, %q) unexpected error: %v", c.dest, c.name, err)
			continue
		}
		rel, relErr := filepath.Rel(filepath.Clean(c.dest), got)
		if relErr != nil || rel == ".." || len(rel) > 2 && rel[:3] == ".."+string(os.PathSeparator) {
			t.Errorf("zipSafeJoin(%q, %q) = %q which escapes the destination", c.dest, c.name, got)
		}
	}
}
