//go:build windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func setupSignalHandlers(signals chan os.Signal, breaksig chan os.Signal) {
	signal.Notify(signals, syscall.SIGINT)

	go func() {
		for s := range signals {
			switch s {
			case syscall.SIGINT:
				breaksig <- syscall.SIGINT
				/*
				   case syscall.SIGUSR1, syscall.SIGBREAK:
				       if !debugger.isInRepl() {
				           pf("\n[#fyellow]Signal %v received! Entering debugger...[#-]\n", s)
				           if activeDebugContext != nil {
				               ifs := activeDebugContext.fs
				               baseIFS := getBaseIFS(ifs)
				               pc := activeDebugContext.pc
				               key := (uint64(ifs) << 32) | uint64(pc)
				               debugger.enterDebugger(key, functionspaces[baseIFS], activeDebugContext.ident, &mident, &gident)
				           } else {
				               pf("[#fred]No active context to enter debugger.[#-]\n")
				           }
				       }
				*/
			}
		}
	}()
}
