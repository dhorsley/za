package main

import (
    "runtime"
	"sort"
    "context"
    "regexp"
	"sync"
	"sync/atomic"
	"time"
    str "strings"
    "strconv"
    "bufio"
    "os"
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

func (d *Debugger) debuggerUnlock() {
    d.lock.Lock()
    d.activeRepl = false
    d.paused = false
    d.lock.Unlock()
}

func (d *Debugger) isInRepl() bool {
    d.lock.RLock()
    defer d.lock.RUnlock()
    return d.activeRepl
}

func getBaseSourceIFS(ifs uint32) uint32 {
    if ifs == 2 { return 1 }
    return getBaseIFS(ifs)
}

func getBaseIFS(ifs uint32) uint32 {
    // Walk the calltable or store source_base per ifs
    // Example (if calltable has .base field):
    // if ifs < uint32(len(calltable)) {
    calllock.RLock()
    defer calllock.RUnlock()
    return calltable[ifs].base
    // }
    // return ifs // fallback
}


func (d *Debugger) enterDebugger(key uint64, statements []Phrase, ident, mident, gident *[]Variable) {

    d.lock.Lock()
    d.paused=true
    d.activeRepl=true
    d.lock.Unlock()
    defer d.debuggerUnlock()

    var ifs uint32

    // Decode key
    ifs = uint32(key >> 32)
    pc := int(key & 0xffffffff)


    // Get true source line from the Phrase
    sourceLine := -1
    sourceBase := getBaseSourceIFS(ifs)

    // Get file name
    filename:=getFileFromIFS(ifs)

    pf("\n[#-]")

    if int(sourceBase) < len(functionspaces) {
        phrases:=functionspaces[sourceBase]
        if pc >= 0 && pc < len(phrases) {
            sourceLine = int(phrases[pc].SourceLine)
        }
        /*
        if phrases!=nil && len(phrases)>0 && pc>0 && pc<len(phrases) {
            pf("[#fblue]Debug: ifs=%d pc=%d phrases.len=%d phrase sourceLine=%d\n[#-]", ifs, pc, len(phrases), phrases[pc].SourceLine)
        } else {
            pf("[#fblue]Debug: phrases is nil or pc out of range! ifs=%d pc=%d phrases.len=%d\n[#-]", ifs, pc, len(phrases))
        }
        */
    }


    display_fs,_:=numlookup.lmget(ifs)

    if key==0 {
        pf("\n[#fred]🛑 Pseudo breakpoint at startup or from interrupt.[#-]\n")
    } else {
        pf("\n[#fred]🛑 Breakpoint hit at %s:%d in function %s[#-]\n", filename, sourceLine, display_fs)
    }

    p:=&leparser{}
    p.ident=ident

    if len(d.watchList) > 0 {
        pf("[#bold]Watched variables at debugger start:[#-]\n")
        for _, w := range d.watchList {
            if val, ok := vget(nil,ifs,ident,w); ok {
                pf("  %s = %v\n", w, val)
            }
        }
    }

    reader := bufio.NewReader(os.Stdin)

    for {
        pf("[#bold][[#3]scope %s : [#6]srcline %05d : [#7]stmtnum %05d] [#5]debug> [#-]",display_fs,sourceLine,pc)
        input, _ := reader.ReadString('\n')
        input = str.TrimSpace(input)

        if input=="" { continue }

        switch {
        case input == "exit":
            input = "quit"
        case input == "where":
            input = "bt"
        }

        switch input {
        case "c", "continue":
            pf("[#fgreen]Continuing execution.[#-]\n")
            d.stepMode = false
            d.nextMode = false
            return

        case "s", "step":
            pf("[#fgreen]Stepping into next statement.[#-]\n")
            d.stepMode = true
            d.nextMode = false
            return

        case "n", "next":
            pf("[#fgreen]Stepping over (next in current function).[#-]\n")
            d.stepMode = false
            d.nextMode = true
            d.nextCallDepth = len(errorChain)
            return


        case "l", "list":
            context := 10 
            pf("[#bold]Source context around PC %d:[#-]\n", pc)

            start := pc - context
            if start < 0 {
                start = 0
            }

            end := pc + context
            if end >= len(statements) {
                end = len(statements) - 1
            }

            for i := start; i <= end; i++ {
                line := statements[i].SourceLine
                lineStr := sf("%04d", line)
                stmtTokens := statements[i].Tokens

                // Check if there is a breakpoint for this line
                key := (uint64(ifs) << 32) | uint64(line)
                d.lock.RLock()
                _, hasBP := d.breakpoints[key]
                d.lock.RUnlock()

                bpMarker := " "  // No BP
                if hasBP {
                    bpMarker = "[#fred]🛑[#-]"
                }

                if i == pc {
                    pf("%s [#fblue]%s[#-] [#fmagenta]* %v[#-]\n", bpMarker, lineStr, stmtTokens)
                } else {
                    pf("%s [#7]%s[#-]   %v[#-]\n", bpMarker, lineStr, stmtTokens)
                }
            }


        case "v", "vars":
            _, err := stdlib["dump"]("", 0, ident)
            if err != nil {
                pf("[#fred]Error: %v[#-]\n", err)
            }

        case "mvars":
            _, err := stdlib["mdump"]("", 0, mident)
            if err != nil {
                pf("[#fred]Error: %v[#-]\n", err)
            }

        case "gvars":
            _, err := stdlib["gdump"]("", 0, gident)
            if err != nil {
                pf("[#fred]Error: %v[#-]\n", err)
            }

        case "p", "print":
            pf("Variable name: ")
            varname, _ := reader.ReadString('\n')
            varname = str.TrimSpace(varname)
            if val, ok := vget(nil,ifs,ident,varname); ok {
                pf("[#bold]%s[#-] = %v\n", varname, val)
            } else {
                pf("[#fred]Variable not found.[#-]\n")
            }

        case "bt", "where":
            pf("[#fblue]Call chain:[#-]\n")
            for i, c := range errorChain {
                pf("  [#fblue]#%d %s[#-]\n", i, c)
            }

        case "fs", "functionspace":
            pf("\n[#6]Debugger entered at %s:%d in function %s[#-]\n", filename, sourceLine, display_fs)


        case "d", "dis":
            pf("[#bold]Show token disassembly of current statement (PC %d):[#-]\n", pc)
            if pc >= 0 && pc < len(statements) {
                for idx, tok := range statements[pc].Tokens {
                    pf("  [#7]%2d[#-] [#fblue]%-12s[#-] [#3]%q[#-]\n", idx, tokNames[tok.tokType], tok.tokText)
                }
            } else {
                pf("[#fred]No statement at current PC![#-]\n")
            }

        case "fn","file":
            currentFile := getFileFromIFS(ifs)
            pf("[#fgreen]Current file: %s[#-]\n", currentFile)

        case "b", "breakpoints":

            pf("[#bold]Current breakpoints:[#-]\n")
            d.lock.RLock()
            for bpKey, cond := range d.breakpoints {
                bpIFS := uint32(bpKey >> 32)
                pc := int(bpKey & 0xffffffff)

                // Determine correct sourceBase for decoding
                sourceBase := getBaseSourceIFS(bpIFS)
                phrases := functionspaces[sourceBase]

                lineNum := -1
                if pc >= 0 && pc < len(phrases) {
                    lineNum = int(phrases[pc].SourceLine)
                }

                file := getFileFromIFS(sourceBase)
                funcName, _ := numlookup.lmget(bpIFS)
                if cond == "" {
                    pf("  %s:%d (%s)\n", file, lineNum, funcName)
                } else {
                    pf("  %s:%d (%s) [#fyellow][if %s][#-]\n", file, lineNum, funcName, cond)
                }
            }
            d.lock.RUnlock()


        case "b+","ba":

            pf("Enter line number for breakpoint: ")
            lineStr, _ := reader.ReadString('\n')
            lineStr = str.TrimSpace(lineStr)

            lineNum, err := strconv.Atoi(lineStr)
            if err != nil {
                pf("[#fred]Invalid line number: %v[#-]\n", err)
                break
            }

            pf("Enter optional condition (or leave blank): ")
            cond, _ := reader.ReadString('\n')
            cond = str.TrimSpace(cond)

            // Use the current ifs for the executing context
            bpIFS := ifs
            sourceBase := getBaseSourceIFS(bpIFS)
            phrases := functionspaces[sourceBase]

            // Find statement index (PC) for lineNum
            pc := -1
            for i, ph := range phrases {
                if int(ph.SourceLine) == lineNum {
                    pc = i
                    break
                }
            }
            if pc == -1 {
                pf("[#fred]Could not find statement for line %d in current file context[#-]\n", lineNum)
                break
            }

            key := (uint64(bpIFS) << 32) | uint64(pc)

            if cond != "" {
                _, err := ev(p, bpIFS, cond)
                if err != nil {
                    pf("[#fred]Error in condition expression: %v[#-]\n", err)
                    break
                }
                pf("[#fyellow]⚠️  Warning: Conditional breakpoints may slow down execution.[#-]\n")
            }

            d.lock.Lock()
            d.breakpoints[key] = cond
            d.lock.Unlock()
            pf("[#fgreen]Breakpoint added at line: %d (0x%x)[#-]\n", lineNum, key)


        case "b-","br":

            pf("Enter line number to remove breakpoint: ")
            lineStr, _ := reader.ReadString('\n')
            lineStr = str.TrimSpace(lineStr)

            lineNum, err := strconv.Atoi(lineStr)
            if err != nil {
                pf("[#fred]Invalid line number: %v[#-]\n", err)
                break
            }

            // Use the current ifs for the executing context
            bpIFS := ifs
            sourceBase := getBaseSourceIFS(bpIFS)
            phrases := functionspaces[sourceBase]

            // Find statement index (PC) for lineNum
            pc := -1
            for i, ph := range phrases {
                if int(ph.SourceLine) == lineNum {
                    pc = i
                    break
                }
            }
            if pc == -1 {
                pf("[#fred]Could not find statement for line %d in current file context[#-]\n", lineNum)
                break
            }

            key := (uint64(bpIFS) << 32) | uint64(pc)

            d.lock.Lock()
            delete(d.breakpoints, key)
            d.lock.Unlock()
            pf("[#fgreen]Breakpoint removed at line: %d (0x%x)[#-]\n", lineNum, key)


        case "cls":
            cls()
            pf("[#-][#CSI]r")

        case "sf", "showf":
            pf("Enter optional function name filter (must include namespace:: prefix): ")
            fnFilter, _ := reader.ReadString('\n')
            fnFilter = str.TrimSpace(fnFilter)
            // ShowDef(fnFilter)
            if fnFilter!="" {
                if val,found:=modlist[fnFilter]; found {
                    if val==true {
                        pf("[#5]Module %s : Functions[#-]\n",fnFilter)
                        for _,fun:=range funcmap {
                            if fun.module==fnFilter {
                                ShowDef(fun.name)
                            }
                        }
                    }
                } else {
                    if _, exists := fnlookup.lmget(fnFilter); exists {
                        ShowDef(fnFilter)
                    } else {
                        pf("Module/function not found.\n")
                    }
                }
            } else {
                fnlookup.m.Range(func(key, value interface{}) bool {
                    name := key.(string)
                    count := value.(uint32)
                    if count < 2 {
                        return true
                    }
                    ShowDef(name)
                    return true
                })
                pf("\n")
            }

        case "ss", "shows":
            pf("Enter optional struct name filter: ")
            structFilter, _ := reader.ReadString('\n')
            structFilter = str.TrimSpace(structFilter)

            for k, s := range structmaps {
                if structFilter != "" {
                    if matched, _ := regexp.MatchString(structFilter, k); !matched {
                        continue
                    }
                }
                pf("[#6]%v[#-]\n", k)
                for i := 0; i < len(s); i += 4 {
                    pf("[#4]%24v[#-] [#3]%v[#-]\n", s[i], s[i+1])
                }
                pf("\n")
            }

        case "w", "watch":
            pf("Enter variable name to watch: ")
            varname, _ := reader.ReadString('\n')
            varname = str.TrimSpace(varname)
            d.watchList = append(d.watchList, varname)
            pf("[#fgreen]Watching variable: %s[#-]\n", varname)

        case "uw", "unwatch":
            pf("Enter variable name to remove from watch list: ")
            varname, _ := reader.ReadString('\n')
            varname = str.TrimSpace(varname)
            newWatch := []string{}
            for _, w := range d.watchList {
                if w != varname {
                    newWatch = append(newWatch, w)
                }
            }
            d.watchList = newWatch
            pf("[#fgreen]Stopped watching variable: %s[#-]\n", varname)

        case "wl", "watchlist":
            pf("[#bold]Watched variables:[#-]\n")
            for _, v := range d.watchList {
                pf("  %s\n", v)
            }

        case "e", "eval":
            pf("Enter expression to evaluate: ")
            expr, _ := reader.ReadString('\n')
            expr = str.TrimSpace(expr)
            result, err := ev(p, ifs, expr)
            if err != nil {
                pf("[#fred]Error: %v[#-]\n", err)
            } else {
                pf("[#fgreen]Result: %v[#-]\n", result)
            }

        case "src", "source":
            pf("Enter file path to source commands from: ")
            file, _ := reader.ReadString('\n')
            file = str.TrimSpace(file)
            data, err := os.ReadFile(file)
            if err != nil {
                pf("[#fred]Error reading file: %v[#-]\n", err)
                break
            }
            commands := str.Split(string(data), "\n")
            for _, cmd := range commands {
                pf("[#bold]debug> %s[#-]\n", cmd)
                input = str.TrimSpace(cmd)
                if input == "" {
                    continue
                }
                if input == "c" || input == "continue" {
                    d.stepMode = false
                    d.nextMode = false
                    return
                }
            }

        case "ton", "traceon":
            lineDebug = true
            pf("[#fgreen]Line-by-line tracing enabled.[#-]\n")

        case "toff", "traceoff":
            lineDebug = false
            pf("[#fgreen]Line-by-line tracing disabled.[#-]\n")

        case "q", "quit":
            pf("[#fred]Exiting interpreter.[#-]\n")
            os.Exit(0)

        case "h", "help":
            pf(`
[#bold]Debugger commands:[#-]
  [#bold]c / continue[#-]       - Resume script execution.
  [#bold]s / step[#-]           - Execute next statement (step into).
  [#bold]n / next[#-]           - Execute next statement in current function (step over).
  [#bold]l / list[#-]           - Show current statement tokens.
  [#bold]v / vars[#-]           - Dump local variables (via stdlib dump()).
  [#bold]mvars[#-]              - Dump module/global-scope variables (via mdump()).
  [#bold]gvars[#-]              - Dump system variables (via gdump()).
  [#bold]sf / showf[#-]         - show function definitions.
  [#bold]ss / shows[#-]         - show struct definitions.
  [#bold]p / print <var>[#-]    - Show a single variable's value.
  [#bold]bt / where[#-]         - Show call chain backtrace.
  [#bold]fn / file[#-]          - Show current file name.
  [#bold]b / breakpoints[#-]    - List all breakpoints.
  [#bold]b+[#-]                 - Add a breakpoint interactively.
  [#bold]b-[#-]                 - Remove a breakpoint.
  [#bold]d / dis[#-]            - Token disassembly.
  [#bold]w / watch[#-]          - Add a variable to watch list.
  [#bold]uw / unwatch[#-]       - Remove a variable from watch list.
  [#bold]wl / watchlist[#-]     - Show watched variables.
  [#bold]e / eval[#-]           - Evaluate an expression in current scope.
  [#bold]src / source[#-]       - Source commands from a file.
  [#bold]ton / traceon[#-]      - Toggle line-by-line tracing on.
  [#bold]toff / traceoff[#-]    - Toggle line-by-line tracing off.
  [#bold]fs / functionspace[#-] - show debug entry point.
  [#bold]cls[#-]                - Toggle line-by-line tracing off.
  [#bold]q / quit / exit[#-]    - Exit the interpreter.
  [#bold]h / help[#-]           - Show this help message.

[#fyellow]Note:[#-]
  The debugger pauses only the main Za script execution.
  Any async subprocesses or shell commands (like those using capture_shell())
  continue to run and may display output directly if not captured.
`)
        default:
            pf("[#fyellow]Type 'help' or 'h' for available commands.[#-]\n")
        }

        if len(d.watchList) > 0 {
            pf("[#bold]Watched variables:[#-]\n")
            for _, w := range d.watchList {
                if val, ok := vget(nil,ifs,ident,w); ok {
                    pf("  %s = %v\n", w, val)
                }
            }
        }
    }
}

