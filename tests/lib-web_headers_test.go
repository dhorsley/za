package main

import (
	"sync"
	"testing"
)

var webHeadersLibOnce sync.Once

func ensureWebHeadersLib() {
	webHeadersLibOnce.Do(buildWebLib)
}

func TestValidWebHeaderName(t *testing.T) {
	for _, name := range []string{"X-Content-Type-Options", "X_Frame_Options", "Cache-Control"} {
		if !validWebHeaderName(name) {
			t.Errorf("expected valid header name: %q", name)
		}
	}
	for _, name := range []string{"", "Bad Header", "X-Test\r\n", "X-Test:"} {
		if validWebHeaderName(name) {
			t.Errorf("expected invalid header name: %q", name)
		}
	}
}

func TestValidWebHeaderValue(t *testing.T) {
	if !validWebHeaderValue("nosniff") {
		t.Fatal("expected simple value to be valid")
	}
	for _, value := range []string{"value\r\nInjected: yes", "value\nInjected: yes"} {
		if validWebHeaderValue(value) {
			t.Errorf("expected invalid header value: %q", value)
		}
	}
}

func TestWebServePathStaticHeaders(t *testing.T) {
	ensureWebHeadersLib()
	fn := stdlib["web_serve_path"]
	if fn == nil {
		t.Fatal("web_serve_path not registered")
	}
	args := []any{"test-static-headers", "s", "/static/", "", map[string]any{
		"headers": map[string]any{
			"X_Content_Type_Options": "nosniff",
			"X_Frame_Options":        "DENY",
		},
	}}
	ok, err := fn("test", 0, nil, args...)
	if err != nil || ok != true {
		t.Fatalf("static headers rejected: ok=%v err=%v", ok, err)
	}
}

func TestWebServePathRejectsInvalidStaticHeader(t *testing.T) {
	ensureWebHeadersLib()
	fn := stdlib["web_serve_path"]
	_, err := fn("test", 0, nil, "test-static-headers-bad", "s", "/static/", "", map[string]any{
		"headers": map[string]any{"Bad Header": "yes"},
	})
	if err == nil {
		t.Fatal("expected invalid static header name to fail")
	}
}

func TestWebServePathWriteMapDoesNotPanic(t *testing.T) {
	ensureWebHeadersLib()
	fn := stdlib["web_serve_path"]
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("write action panicked: %v", r)
		}
	}()
	_, err := fn("test", 0, nil, "test-write-map", "w", "/x/", "a", map[string]any{"headers": map[string]any{"X-Test": "yes"}})
	if err == nil {
		t.Fatal("expected write action with a map replacement to fail")
	}
}
