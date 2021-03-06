// +build !windows freebsd linux

package main

import (
    "syscall"
    "os"
    "os/signal"
)

func setWinchSignal(sigs chan os.Signal) {
    signal.Notify(sigs, syscall.SIGWINCH)
}

