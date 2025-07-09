// stringmap_test.go
package main

import "testing"

func TestLmapBasicCRUD(t *testing.T) {
    lm := nlmcreate(0)
    var key uint32 = 42

    // Initially not exist
    if lm.lmexists(key) {
        t.Fatalf("expected key %d not to exist", key)
    }
    // lmget → ("", false)
    if v, ok := lm.lmget(key); ok || v != "" {
        t.Fatalf("expected lmget(%d)==(\"\",false), got (%q,%v)", key, v, ok)
    }

    // lmset and lmget
    lm.lmset(key, "hello")
    if !lm.lmexists(key) {
        t.Fatalf("expected key %d to exist after set", key)
    }
    if v, ok := lm.lmget(key); !ok || v != "hello" {
        t.Fatalf("expected lmget(%d)==(\"hello\",true), got (%q,%v)", key, v, ok)
    }

    // Overwrite
    lm.lmset(key, "world")
    if v, ok := lm.lmget(key); !ok || v != "world" {
        t.Fatalf("expected lmget(%d)==(\"world\",true), got (%q,%v)", key, v, ok)
    }

    // Delete nonexistent
    if deleted := lm.lmdelete(key + 1); deleted {
        t.Fatalf("lmdelete(%d) should return false", key+1)
    }
    // Delete existing
    if deleted := lm.lmdelete(key); !deleted {
        t.Fatalf("lmdelete(%d) should return true", key)
    }
    if lm.lmexists(key) {
        t.Fatalf("key %d should not exist after deletion", key)
    }
}

