package main

import (
    "runtime"
	"sort"
    "context"
	"sync"
	"sync/atomic"
	"time"
    str "strings"
)

type profilerKeyType struct{}

var (
	profileMu       sync.Mutex
	profiles        = make(map[string]*ProfileContext)
	enableProfiling bool // set via flag
    profilerKey     = profilerKeyType{}
    profileCallChains sync.Map
)

type ProfileContext struct {
	Times map[string]time.Duration
}

var nextProfileID uint64 = 1

func withProfilerContext(parent context.Context) context.Context {
    id := atomic.AddUint64(&nextProfileID, 1)
    return context.WithValue(parent, profilerKey, id)
}


// getGoroutineID returns the current goroutine's unique ID
func getGoroutineID() uint64 {
    // Use runtime.Stack to extract the goroutine ID
    var buf [64]byte
    n := runtime.Stack(buf[:], false)
    var id uint64
    for _, b := range buf[:n] {
        if b >= '0' && b <= '9' {
            id = id*10 + uint64(b-'0')
        } else if id > 0 {
            break
        }
    }
    return id
}

func getCallChain(ctx context.Context) []string {
    id, ok := ctx.Value(profilerKey).(uint64)
    if !ok {
        return []string{}
    }
    if v, ok := profileCallChains.Load(id); ok {
        return v.([]string)
    }
    return []string{}
}

func setCallChain(ctx context.Context, chain []string) {
    id, ok := ctx.Value(profilerKey).(uint64)
    if !ok {
        return
    }
    profileCallChains.Store(id, chain)
}

func pushToCallChain(ctx context.Context, name string) {
    chain := getCallChain(ctx)
    chain = append(chain, name)
    setCallChain(ctx, chain)
}

func popCallChain(ctx context.Context) {
    chain := getCallChain(ctx)
    if len(chain) > 0 {
        chain = chain[:len(chain)-1]
    }
    setCallChain(ctx, chain)
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

func stopProfile(name string, startTime time.Time) {
    if !enableProfiling {
        return
    }

    // Lock to safely update the profile map
    profileMu.Lock()
    defer profileMu.Unlock()

    // Collapse the current call chain to form the path key
    pathKey := name

    // Check if the profile entry exists
    ctx, exists := profiles[pathKey]
    if !exists {
        // Create a new profile entry if it doesn’t exist
        ctx = &ProfileContext{Times: make(map[string]time.Duration)}
        profiles[pathKey] = ctx
    }

    // Record the exclusive execution time for this profile
    duration := time.Since(startTime)
    if d, ok := ctx.Times[name]; ok {
        ctx.Times[name] = d + duration
    } else {
        ctx.Times[name] = duration
    }
}

// Called inside any phase you want to profile
func recordPhase(ctx context.Context, phase string, elapsed time.Duration) {
    if !enableProfiling {
        return
    }

    callChain:=getCallChain(ctx)

    pathKey := collapseCallPath(callChain)

    profileMu.Lock()
    if _, exists := profiles[pathKey]; !exists {
        profiles[pathKey] = &ProfileContext{Times: make(map[string]time.Duration)}
    }
    profiles[pathKey].Times[phase] += elapsed
    profileMu.Unlock()
}

func recordExclusiveExecutionTime(ctx context.Context, callChain []string, elapsed time.Duration) {
    if !enableProfiling {
        return
    }

    pathKey := collapseCallPath(getCallChain(ctx))

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

    pf("\n[#bold][#5]Profile Summary[#boff][#-]\n\n")

    var keys []string
    for k := range profiles {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    for _, path := range keys {

        if len(path)==0 {
            continue
        }

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
            pf("%s[#bold][#2]%s[#-] (recursive [unreliable timings]):\n", indent, path)
        } else {
            pf("%s[#bold][#4]%s[#-]:\n", indent, path)
        }

        for phase, t := range p.Times {
            if phase=="recursive" {
                continue
            }
            colour:="[#1]"
            if phase=="execution time" { colour="[#6]" }
            pf("%s "+colour+"%s[#-]: %v\n", indent, phase, t)
        }
        pln()
    }
}

