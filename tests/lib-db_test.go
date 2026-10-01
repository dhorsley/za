package main

import (
	"os"
	"strings"
	"testing"
)

func init() {
	buildStandardLib()
}

func TestDbInitUnknownEngine(t *testing.T) {
	t.Setenv("ZA_DB_ENGINE", "oracle")
	t.Setenv("ZA_DB_HOST", "localhost")
	t.Setenv("ZA_DB_PORT", "1521")
	t.Setenv("ZA_DB_USER", "u")
	t.Setenv("ZA_DB_PASS", "p")

	got, err := stdlib["db_init"]("", 0, nil, "schema")
	if err == nil {
		t.Fatalf("db_init with unknown engine returned nil error (got %v)", got)
	}
	if !strings.Contains(err.Error(), "unsupported DB engine") {
		t.Errorf("db_init error = %q, want mention of unsupported engine", err.Error())
	}
}

func TestDbInitMissingEngine(t *testing.T) {
	prev, had := os.LookupEnv("ZA_DB_ENGINE")
	os.Unsetenv("ZA_DB_ENGINE")
	defer func() {
		if had {
			os.Setenv("ZA_DB_ENGINE", prev)
		}
	}()

	got, err := stdlib["db_init"]("", 0, nil, "schema")
	if err == nil {
		t.Fatalf("db_init with no engine set returned nil error (got %v)", got)
	}
	if !strings.Contains(err.Error(), "No DB engine specified") {
		t.Errorf("db_init error = %q, want mention of missing engine", err.Error())
	}
}

func TestDbInitSqlite3(t *testing.T) {
	t.Setenv("ZA_DB_ENGINE", "sqlite3")

	got, err := stdlib["db_init"]("", 0, nil, ":memory:")
	if err != nil {
		t.Fatalf("db_init with sqlite3 failed: %v", err)
	}
	if got == nil {
		t.Fatal("db_init with sqlite3 returned nil handle")
	}

	if _, err := stdlib["db_close"]("", 0, nil, got); err != nil {
		t.Errorf("db_close failed: %v", err)
	}
}
