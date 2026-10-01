package main

import (
	"fmt"
	"testing"
	"time"
)

func init() {
	buildStandardLib()
}

func resetWebCacheForTest() {
	web_cache_lock.Lock()
	web_cache = make(map[string]web_cache_entry)
	web_cache_lock.Unlock()
}

func TestWebCacheMemoryEnforcement(t *testing.T) {
	resetWebCacheForTest()
	prev := web_cache_max_memory
	defer func() {
		web_cache_max_memory = prev
		resetWebCacheForTest()
	}()

	web_cache_max_memory = 1 // 1 MB cap

	web_cache_lock.Lock()
	now := time.Now()
	for i := 0; i < 3; i++ {
		web_cache[fmt.Sprintf("k%d", i)] = web_cache_entry{
			content:   make([]byte, 512*1024), // 512 KB each
			timestamp: now.Add(time.Duration(i) * time.Second),
		}
	}
	// 1.5 MB total > 1 MB cap -> oldest (k0) evicted, 1.0 MB remains
	webCacheEnforceMemory()
	remaining := len(web_cache)
	web_cache_lock.Unlock()

	if remaining != 2 {
		t.Errorf("after enforcement cache has %d entries, want 2", remaining)
	}

	if _, ok := web_cache["k0"]; ok {
		t.Error("oldest entry k0 should have been evicted")
	}
	if _, ok := web_cache["k2"]; !ok {
		t.Error("newest entry k2 should have been kept")
	}
}

func TestWebCacheMemoryUnlimitedWhenZero(t *testing.T) {
	resetWebCacheForTest()
	prev := web_cache_max_memory
	defer func() {
		web_cache_max_memory = prev
		resetWebCacheForTest()
	}()

	web_cache_max_memory = 0 // unlimited

	web_cache_lock.Lock()
	now := time.Now()
	for i := 0; i < 3; i++ {
		web_cache[fmt.Sprintf("k%d", i)] = web_cache_entry{
			content:   make([]byte, 512*1024),
			timestamp: now.Add(time.Duration(i) * time.Second),
		}
	}
	webCacheEnforceMemory()
	remaining := len(web_cache)
	web_cache_lock.Unlock()

	if remaining != 3 {
		t.Errorf("with unlimited cap cache has %d entries, want 3", remaining)
	}
}

func TestWebCacheCleanupEnforcesMemory(t *testing.T) {
	resetWebCacheForTest()
	prevAge := web_cache_max_age
	prevMem := web_cache_max_memory
	defer func() {
		web_cache_max_age = prevAge
		web_cache_max_memory = prevMem
		resetWebCacheForTest()
	}()

	web_cache_max_age = 3600  // nothing expires by age
	web_cache_max_memory = 1   // 1 MB cap

	web_cache_lock.Lock()
	now := time.Now()
	for i := 0; i < 4; i++ {
		web_cache[fmt.Sprintf("k%d", i)] = web_cache_entry{
			content:   make([]byte, 512*1024),
			timestamp: now, // fresh: age eviction must not trigger
		}
	}
	web_cache_lock.Unlock()

	cleanupWebCache()

	web_cache_lock.Lock()
	remaining := len(web_cache)
	web_cache_lock.Unlock()

	if remaining != 2 {
		t.Errorf("cleanupWebCache left %d entries, want 2 (memory cap)", remaining)
	}
}

func TestWebCacheStatsMemory(t *testing.T) {
	resetWebCacheForTest()
	prev := web_cache_max_memory
	defer func() {
		web_cache_max_memory = prev
		resetWebCacheForTest()
	}()

	web_cache_max_memory = 42

	web_cache_lock.Lock()
	web_cache["a"] = web_cache_entry{content: make([]byte, 1000), timestamp: time.Now()}
	web_cache["b"] = web_cache_entry{content: make([]byte, 24), timestamp: time.Now()}
	web_cache_lock.Unlock()

	got, err := stdlib["web_cache_stats"]("", 0, nil)
	if err != nil {
		t.Fatalf("web_cache_stats failed: %v", err)
	}
	stats, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("web_cache_stats returned %T, want map", got)
	}

	if stats["memory_bytes"] != int64(1024) {
		t.Errorf("memory_bytes = %v, want 1024", stats["memory_bytes"])
	}
	if stats["max_memory_mb"] != 42 {
		t.Errorf("max_memory_mb = %v, want 42", stats["max_memory_mb"])
	}
	if stats["size"] != 2 {
		t.Errorf("size = %v, want 2", stats["size"])
	}
}
