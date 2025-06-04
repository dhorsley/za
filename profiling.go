package main

import (
	"fmt"
	"sort"
	"sync"
	"time"
    str "strings"
)

var (
	profileMu       sync.Mutex
	profiles        = make(map[string]*ProfileContext)
	enableProfiling bool // set via flag
)

type ProfileContext struct {
	Times map[string]time.Duration
}


func isRecursive(callChain []string) bool {
    seen := make(map[string]bool)
    for _, f := range callChain {
        // Strip the @ suffix if present
        baseName := f
        if idx := str.Index(f, "@"); idx != -1 {
            baseName = f[:idx]
        }

        if seen[baseName] {
            return true // real recursion!
        }
        seen[baseName] = true
    }
    return false
}


func buildCallPathKey(callChain []string) string {
    return str.Join(callChain, " > ")
}

func collapseCallPath(callChain []string) string {
    if len(callChain) == 0 {
        return ""
    }

    // Extract the function name up to @ for comparison
    lastFull := callChain[len(callChain)-1]
    last := lastFull
    if idx := str.Index(lastFull, "@"); idx != -1 {
        last = lastFull[:idx]
    }

    collapsed := []string{}
    for _, c := range callChain {
        name := c
        if idx := str.Index(c, "@"); idx != -1 {
            name = c[:idx]
        }
        if name != last {
            collapsed = append(collapsed, c)
        }
    }

    // collapsed = append(collapsed, last+" (recursive)") // [unreliable timings])")
    collapsed = append(collapsed, last)
    return str.Join(collapsed, " > ")
}


// Called at start of a function or script
func startProfile(caller string) {
	if !enableProfiling {
		return
	}
	profileMu.Lock()
	if _, exists := profiles[caller]; !exists {
		profiles[caller] = &ProfileContext{Times: make(map[string]time.Duration)}
	}
	profileMu.Unlock()
}

// Called inside any phase you want to profile
func recordPhase(callChain []string, phase string, elapsed time.Duration) {
    if !enableProfiling {
        return
    }

    // pathKey := buildCallPathKey(callChain)
    pathKey := collapseCallPath(callChain)

    profileMu.Lock()
    if _, exists := profiles[pathKey]; !exists {
        profiles[pathKey] = &ProfileContext{Times: make(map[string]time.Duration)}
    }
    profiles[pathKey].Times[phase] += elapsed
    profileMu.Unlock()
}

func recordExclusiveExecutionTime(callChain []string, elapsed time.Duration) {
    if !enableProfiling {
        return
    }

    pathKey := collapseCallPath(callChain)

    profileMu.Lock()
    if _, exists := profiles[pathKey]; !exists {
        profiles[pathKey] = &ProfileContext{Times: make(map[string]time.Duration)}
    }
    profiles[pathKey].Times["execution time"] += elapsed
    profileMu.Unlock()
}

// Print summary at program end
func dumpProfileSummary() {
    profileMu.Lock()
    defer profileMu.Unlock()

    fmt.Println("Profile Summary:")

    var keys []string
    for k := range profiles {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    for _, path := range keys {

        p:=profiles[path]

        isRecursive:= p.Times["recursive"] > 0

        hasNonZero:=false
        for _,t:=range p.Times {
            if t>0 {
                hasNonZero=true
                break
            }
        }
        if !hasNonZero {
            continue
        }

        indentLevel := str.Count(path, ">")
        indent := str.Repeat("  ", indentLevel)


        if isRecursive {
            fmt.Printf("%s%s (recursive [unreliable timings]):\n", indent, path)
        } else {
            fmt.Printf("%s%s:\n", indent, path)
        }

        for phase, t := range p.Times {
            if phase=="recursive" {
                continue
            }
            fmt.Printf("%s  %s: %v\n", indent, phase, t)
        }
    }
}

