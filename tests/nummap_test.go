// nummap_test.go
package main

import "testing"

func TestNmapBasicCRUD(t *testing.T) {
	nm := nlmcreate(0)
	var key uint32 = 7

	if nm.lmexists(key) {
		t.Fatalf("expected key %d not to exist", key)
	}
	if v, ok := nm.lmget(key); ok || v != "" {
		t.Fatalf("expected lmget(%d)==(\"\",false), got (%q,%v)", key, v, ok)
	}

	nm.lmset(key, "seven")
	if !nm.lmexists(key) {
		t.Fatalf("expected key %d to exist", key)
	}
	if v, ok := nm.lmget(key); !ok || v != "seven" {
		t.Fatalf("expected lmget(%d)==(\"seven\",true), got (%q,%v)", key, v, ok)
	}

	nm.lmset(key, "SEVEN")
	if v, ok := nm.lmget(key); !ok || v != "SEVEN" {
		t.Fatalf("expected lmget(%d)==(\"SEVEN\",true), got (%q,%v)", key, v, ok)
	}

	if deleted := nm.lmdelete(key + 1); deleted {
		t.Fatalf("lmdelete(%d) should return false", key+1)
	}
	if deleted := nm.lmdelete(key); !deleted {
		t.Fatalf("lmdelete(%d) should return true", key)
	}
	if nm.lmexists(key) {
		t.Fatalf("key %d should not exist after deletion", key)
	}
}

