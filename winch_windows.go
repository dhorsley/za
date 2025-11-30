//go:build windows

package main

import (
    "os"
)

func setWinchSignal(sigs chan os.Signal) {
    // do nothing
}
