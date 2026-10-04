//go:build linux || freebsd || openbsd || netbsd || dragonfly

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func init() {
	buildStandardLib()
}

func TestSendSignalToSelf(t *testing.T) {
	pid := os.Getpid()

	// Signal 0 checks if process exists, no actual signal sent
	got, err := stdlib["send_signal"]("", 0, nil, pid, 0)
	if err != nil {
		t.Fatalf("send_signal(0) failed: %v", err)
	}
	if !got.(bool) {
		t.Errorf("send_signal(0) to self = false, want true")
	}

	// SIGUSR1 should succeed for ourselves
	got, err = stdlib["send_signal"]("", 0, nil, pid, "USR1")
	if err != nil {
		t.Fatalf("send_signal(USR1) failed: %v", err)
	}
	if !got.(bool) {
		t.Errorf("send_signal(USR1) to self = false, want true")
	}
}

func TestSendSignalNonExistent(t *testing.T) {
	// Non-existent PID should return false, not error
	got, err := stdlib["send_signal"]("", 0, nil, 999999, "KILL")
	if err != nil {
		t.Fatalf("send_signal to non-existent process failed: %v", err)
	}
	if got.(bool) {
		t.Errorf("send_signal to non-existent process = true, want false")
	}
}

func TestSendSignalInvalidName(t *testing.T) {
	pid := os.Getpid()
	_, err := stdlib["send_signal"]("", 0, nil, pid, "INVALID_SIGNAL")
	if err == nil {
		t.Errorf("send_signal with invalid name should have failed")
	}
}

func TestSendSignalByNumber(t *testing.T) {
	// Create a child process that sleeps
	cmd := exec.Command("sleep", "10")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start child: %v", err)
	}
	defer cmd.Process.Kill()

	childPid := cmd.Process.Pid
	time.Sleep(100 * time.Millisecond)

	// Test numeric signal 1 (SIGHUP) to child
	got, err := stdlib["send_signal"]("", 0, nil, childPid, 1)
	if err != nil {
		t.Fatalf("send_signal(1) failed: %v", err)
	}
	if !got.(bool) {
		t.Errorf("send_signal(1) = false, want true")
	}

	// Create a second child for SIGKILL test
	cmd2 := exec.Command("sleep", "10")
	err = cmd2.Start()
	if err != nil {
		t.Fatalf("Failed to start second child: %v", err)
	}
	defer cmd2.Process.Kill()

	childPid2 := cmd2.Process.Pid
	time.Sleep(100 * time.Millisecond)

	// Test numeric signal 9 (SIGKILL) to child
	got, err = stdlib["send_signal"]("", 0, nil, childPid2, 9)
	if err != nil {
		t.Fatalf("send_signal(9) failed: %v", err)
	}
	if !got.(bool) {
		t.Errorf("send_signal(9) = false, want true")
	}

	// Wait for child to die from SIGKILL
	cmd2.Wait()
}

func TestSendSignalToChild(t *testing.T) {
	// Create a child process that sleeps
	cmd := exec.Command("sleep", "10")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start child: %v", err)
	}
	defer cmd.Process.Kill()

	childPid := cmd.Process.Pid

	// Give child a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check child exists with signal 0
	got, err := stdlib["send_signal"]("", 0, nil, childPid, 0)
	if err != nil {
		t.Fatalf("send_signal(0) to child failed: %v", err)
	}
	if !got.(bool) {
		t.Fatalf("Child process %d does not exist", childPid)
	}

	// Send SIGTERM to child
	got, err = stdlib["send_signal"]("", 0, nil, childPid, "TERM")
	if err != nil {
		t.Fatalf("send_signal(TERM) to child failed: %v", err)
	}
	if !got.(bool) {
		t.Errorf("send_signal(TERM) to child = false, want true")
	}

	// Wait for child to exit
	cmd.Wait()

	// After child exits, signal should return false
	got, err = stdlib["send_signal"]("", 0, nil, childPid, 0)
	if err != nil {
		t.Fatalf("send_signal(0) to exited child failed: %v", err)
	}
	if got.(bool) {
		t.Errorf("send_signal(0) to exited child = true, want false")
	}
}

func TestSendSignalPermissionDenied(t *testing.T) {
	// Send signal to PID 1 (init) - should fail with EPERM if not root
	got, err := stdlib["send_signal"]("", 0, nil, 1, "HUP")
	if err != nil {
		t.Fatalf("send_signal to PID 1 failed: %v", err)
	}
	// We expect false unless running as root
	if got.(bool) {
		// Running as root - check if we can verify it actually worked
		t.Logf("send_signal to PID 1 succeeded (running as root?)")
	}
}

func TestPsInfoOptionsAccepted(t *testing.T) {
	// The documented two argument form ps_info(pid, options) must be accepted.
	// The expect_args spec previously declared the second variant with an
	// arity of 1, which made every two argument call unreachable.
	pid := os.Getpid()

	if _, err := stdlib["ps_info"]("", 0, nil, pid); err != nil {
		t.Fatalf("ps_info(pid) failed: %v", err)
	}

	if _, err := stdlib["ps_info"]("", 0, nil, pid, map[string]any{}); err != nil {
		t.Fatalf("ps_info(pid, options) rejected an empty options map: %v", err)
	}

	if _, err := stdlib["ps_info"]("", 0, nil, pid, map[string]any{"include_environ": false}); err != nil {
		t.Fatalf("ps_info(pid, .include_environ false) failed: %v", err)
	}
}

func TestPsInfoEnviron(t *testing.T) {
	pid := os.Getpid()

	// Off by default
	got, err := stdlib["ps_info"]("", 0, nil, pid)
	if err != nil {
		t.Fatalf("ps_info failed: %v", err)
	}
	if len(got.(ProcessInfo).Environ) != 0 {
		t.Errorf("Environ populated without .include_environ, got %d entries",
			len(got.(ProcessInfo).Environ))
	}

	// On - our own environ is always readable
	got, err = stdlib["ps_info"]("", 0, nil, pid, map[string]any{"include_environ": true})
	if err != nil {
		t.Fatalf("ps_info with .include_environ failed: %v", err)
	}
	env := got.(ProcessInfo).Environ
	if len(env) == 0 {
		t.Fatalf("Environ empty for own pid %d, want at least one entry", pid)
	}
	for _, kv := range env {
		if kv == "" {
			t.Errorf("Environ contains an empty entry")
			break
		}
		if !strings.Contains(kv, "=") {
			t.Errorf("Environ entry %q is not a KEY=VALUE pair", kv)
			break
		}
	}
}

func TestDiskUsageExcludePatterns(t *testing.T) {
	all, err := getDiskUsage(nil)
	if err != nil {
		t.Skipf("getDiskUsage unavailable: %v", err)
	}
	if len(all) == 0 {
		t.Skip("no mounts reported, nothing to filter")
	}

	// Pick a filesystem type that is actually present so the assertion is
	// meaningful on any host.
	var target string
	for _, m := range all {
		if m["fstype"] != nil {
			target = m["fstype"].(string)
			break
		}
	}
	if target == "" {
		t.Skip("no fstype available to filter on")
	}

	filtered, err := getDiskUsage(map[string]any{"exclude_patterns": []any{target}})
	if err != nil {
		t.Fatalf("getDiskUsage with exclude_patterns failed: %v", err)
	}

	if len(filtered) >= len(all) {
		t.Errorf("exclude_patterns [%q] did not reduce the mount count: %d -> %d",
			target, len(all), len(filtered))
	}

	// Nothing that was excluded may survive.
	for _, m := range filtered {
		fs, _ := m["fstype"].(string)
		mp, _ := m["mounted_path"].(string)
		if fs == target || mp == target {
			t.Errorf("excluded mount %q (%s) still present", mp, fs)
		}
	}
}

func TestMatchesAnyPattern(t *testing.T) {
	cases := []struct {
		patterns any
		values   []string
		want     bool
	}{
		{[]any{"tmpfs"}, []string{"proc", "tmpfs"}, true},
		{[]any{"tmpfs"}, []string{"proc", "sysfs"}, false},
		{[]string{"proc"}, []string{"proc"}, true},
		{[]any{"nomatch"}, []string{"proc", "sysfs"}, false},
		{[]any{1, "proc"}, []string{"proc"}, true}, // non string entries skipped
		{[]any{}, []string{"proc"}, false},
		{"notalist", []string{"proc"}, false},
	}

	for _, c := range cases {
		if got := matchesAnyPattern(c.patterns, c.values...); got != c.want {
			t.Errorf("matchesAnyPattern(%#v, %v) = %v, want %v",
				c.patterns, c.values, got, c.want)
		}
	}
}
