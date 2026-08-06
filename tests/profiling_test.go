package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDumpProfileEventsIsSpeedscopeJSON(t *testing.T) {
	old := enableProfileEvents
	enableProfileEvents = true
	profileEventsMu.Lock()
	profileTracks = make(map[uint64][]profileEvent)
	profileFrames = make(map[string]int)
	profileStart = time.Now()
	profileEventsMu.Unlock()
	recordProfileEvent(true, "main", profileStart)
	recordProfileEvent(false, "main", profileStart.Add(time.Millisecond))
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := dumpProfileEvents(path); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["$schema"] == nil || got["profiles"] == nil {
		t.Fatal("missing Speedscope fields")
	}
	enableProfileEvents = old
}
