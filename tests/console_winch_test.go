// winch_test.go
package main

import (
    "os"
    "syscall"
    "testing"
    "time"
)

func TestSetWinchSignal(t *testing.T) {
    // 1) Create a buffered channel for os.Signal
    sigs := make(chan os.Signal, 1)

    // 2) Call setWinchSignal so that it does something like
    //    signal.Notify(sigs, syscall.SIGWINCH) under the hood
    setWinchSignal(sigs)

    // 3) Send SIGWINCH to ourselves
    pid := os.Getpid()
    if err := syscall.Kill(pid, syscall.SIGWINCH); err != nil {
        t.Fatalf("failed to send SIGWINCH: %v", err)
    }

    // 4) Wait up to 1 second for that signal to arrive on sigs
    select {
    case s := <-sigs:
        if s != syscall.SIGWINCH {
            t.Fatalf("expected signal SIGWINCH, got %v", s)
        }
    case <-time.After(1 * time.Second):
        t.Fatal("timeout waiting for SIGWINCH")
    }
}

