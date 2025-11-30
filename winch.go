//go:build !windows || freebsd || linux

package main

import (
    "os"
    "os/signal"
    "syscall"
)

func setWinchSignal(sigs chan os.Signal) {
    signal.Notify(sigs, syscall.SIGWINCH)
}
