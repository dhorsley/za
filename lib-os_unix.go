// +build !windows

package main

import (
    "syscall"
    "os"
)

func fileStatSys(fp string) (*syscall.Stat_t) {
    f, err := os.Stat(fp)
    if err==nil {
        return f.Sys().(*syscall.Stat_t)
    }
    return nil
}

