package main

// Unit tests for internal Prometheus counters backing the 1.3.0
// concurrency work (locks, yield/emit/resume/drain, list writes, unset).
// Each test snapshots the counter, exercises the path, and asserts the
// exact delta. enableMetrics is toggled per test (saved/restored).

import (
    "testing"

    "github.com/VictoriaMetrics/metrics"
)

func withMetrics(t *testing.T, f func()) {
    t.Helper()
    old := enableMetrics
    enableMetrics = true
    buildStandardLib()
    defer func() { enableMetrics = old }()
    f()
}

func TestLockCounters(t *testing.T) {
    withMetrics(t, func() {
        beforeAcquire := metrics.GetOrCreateCounter(`za_lock_acquire_total`).Get()
        beforeTimeout := metrics.GetOrCreateCounter(`za_lock_timeout_total`).Get()
        beforeErrors := metrics.GetOrCreateCounter(`za_lock_errors_total`).Get()
        beforeHold := metrics.GetOrCreateSummary(`za_lock_hold_ms`)

        lockFn := stdlib["lock"]
        unlockFn := stdlib["unlock"]
        trylockFn := stdlib["trylock"]

        if _, err := lockFn("", 0, nil, "ut-m1", 1000); err != nil {
            t.Fatalf("lock failed: %v", err)
        }
        if _, err := trylockFn("", 0, nil, "ut-m2"); err != nil {
            t.Fatalf("trylock failed: %v", err)
        }
        // held by us: timed wait must expire, trylock must refuse
        if r, _ := lockFn("", 0, nil, "ut-m1", 20); r != false {
            t.Fatalf("timed lock on held mutex should be false")
        }
        if r, _ := trylockFn("", 0, nil, "ut-m2"); r != false {
            t.Fatalf("trylock on held mutex should be false")
        }
        if _, err := unlockFn("", 0, nil, "ut-m1"); err != nil {
            t.Fatalf("unlock failed: %v", err)
        }
        if _, err := unlockFn("", 0, nil, "ut-m2"); err != nil {
            t.Fatalf("unlock failed: %v", err)
        }
        // misuse paths
        if r, _ := unlockFn("", 0, nil, "ut-never"); r != false {
            t.Fatalf("unknown unlock should be false")
        }
        if r, _ := unlockFn("", 0, nil, "ut-m1"); r != false {
            t.Fatalf("double unlock should be false")
        }

        if d := metrics.GetOrCreateCounter(`za_lock_acquire_total`).Get() - beforeAcquire; d != 2 {
            t.Errorf("acquire delta = %d, want 2", d)
        }
        if d := metrics.GetOrCreateCounter(`za_lock_timeout_total`).Get() - beforeTimeout; d != 1 {
            t.Errorf("timeout delta = %d, want 1", d)
        }
        if d := metrics.GetOrCreateCounter(`za_lock_errors_total`).Get() - beforeErrors; d != 2 {
            t.Errorf("errors delta = %d, want 2", d)
        }
        _ = beforeHold
    })
}

func TestListWriteCounter(t *testing.T) {
    withMetrics(t, func() {
        before := metrics.GetOrCreateCounter(`za_list_write_total`).Get()
        concatFn := stdlib["concat"]
        if _, err := concatFn("", 0, nil, []float32{1, 2}, []float32{3}); err != nil {
            t.Fatalf("concat failed: %v", err)
        }
        appendFn := stdlib["append"]
        if _, err := appendFn("", 0, nil, []int{1}, 2); err != nil {
            t.Fatalf("append failed: %v", err)
        }
        if d := metrics.GetOrCreateCounter(`za_list_write_total`).Get() - before; d != 2 {
            t.Errorf("list-write delta = %d, want 2", d)
        }
    })
}

func TestMetricsDisabledByDefault(t *testing.T) {
    // Emissions must be silent unless explicitly enabled: force off and
    // assert counters don't move.
    old := enableMetrics
    enableMetrics = false
    buildStandardLib()
    defer func() { enableMetrics = old }()
    before := metrics.GetOrCreateCounter(`za_lock_acquire_total`).Get()
    lockFn := stdlib["lock"]
    if _, err := lockFn("", 0, nil, "ut-off"); err != nil {
        t.Fatalf("lock failed: %v", err)
    }
    if _, err := stdlib["unlock"]("", 0, nil, "ut-off"); err != nil {
        t.Fatalf("unlock failed: %v", err)
    }
    if d := metrics.GetOrCreateCounter(`za_lock_acquire_total`).Get() - before; d != 0 {
        t.Errorf("disabled metrics still counted: delta = %d", d)
    }
}
