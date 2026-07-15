//go:build linux || freebsd || openbsd || netbsd || dragonfly

package main

import (
	"os"
	"os/exec"
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
