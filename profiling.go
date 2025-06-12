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
    "path/filepath"
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

func humanReadableSize(size int64) string {
    const unit = 1024
    if size < unit {
        return sf("%d B", size)
    }
    div, exp := int64(unit), 0
    for n := size / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return sf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
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
    calllock.RLock()
    defer calllock.RUnlock()
    return calltable[ifs].base
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

    // pf("Inside enterDebugger. From key decode : ifs=%d pc=%d\n",ifs,pc)

    // Get positional details
    calllock.RLock()
    filename    := getFileFromIFS(ifs)
    sourceBase  := calltable[ifs].base
    calllock.RUnlock()

    // sourceBase  := getBaseSourceIFS(ifs)
    phrases     := functionspaces[sourceBase]
    display_fs,_:= numlookup.lmget(ifs)
    sourceLine  := int16(-1)
    if pc>=0 && len(phrases)>pc {
        sourceLine  = int16(phrases[pc].SourceLine)
    }

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
        pf("[#bold][[#3]scope %s : [#6]line %05d : [#7]idx %05d] [#5]debug> [#-]",display_fs,sourceLine,pc)
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

        case "ctx", "context":
            pf("Current context range for listing is: %d\n", d.listContext)
            pf("Enter new context range (default: 10): ")
            input, _ := reader.ReadString('\n')
            input = str.TrimSpace(input)
            if input == "" {
                pf("[#fyellow]No change made.[#-]\n")
                break
            }
            newCtx, err := strconv.Atoi(input)
            if err != nil || newCtx < 0 {
                pf("[#fred]Invalid context value. Please enter a positive number.[#-]\n")
                break
            }
            d.listContext = newCtx
            pf("[#fgreen]Updated context range to: %d[#-]\n", d.listContext)


        case "l", "list":

            // Determine phrase list and context window
            start := pc - d.listContext
            if start < 0 {
                start = 0
            }
            end := pc + d.listContext
            if end >= len(phrases) {
                end = len(phrases) - 1
            }

            // Calculate relative file path
            filePath := filename
            // getFileFromIFS(ifs)
            cwd, _ := os.Getwd()
            relPath, err := filepath.Rel(cwd, filePath)
            if err != nil {
                relPath = filePath // fallback
            }

            // Calculate human-readable file size
            fileStat, _ := os.Stat(filePath)
            var fileSize string
            if fileStat != nil {
                fileSize = humanReadableSize(fileStat.Size())
            } else {
                fileSize = "unknown"
            }

            // Header line
            pf("\n[#bold]Source file: %s (size: %s)[#-]\n", relPath, fileSize)
            pf("[#bold]Source context around PC %d:[#-]\n", pc)
            header := sf("[#6]  %-5s %-3s %-5s %-3s %s[#-]", "IDX", "BP", "LINE", "CUR", "TOKENS")
            pf(header + "\n")

            // Calculate max visible width of rows
            maxWidth := len(Strip(StripCC(header)))
            rows := []string{}

            for i := start; i <= end; i++ {
                phrase := phrases[i]
                line := phrase.SourceLine

                // Check for breakpoint marker
                bpMarker := "   "
                if _, hasBP := debugger.breakpoints[(uint64(ifs)<<32)|uint64(i)]; hasBP {
                    bpMarker = sparkle("[#fred] ● [#.]")
                }

                scope := "   "
                if line == sourceLine {
                    scope = sparkle("[#fblue] ★ [#.]")
                }

                // Convert Tokens slice to a plain string list
                tokens := []string{}
                for _, t := range phrase.Tokens {
                    tokens = append(tokens, sf("%v",t))
                }
                tokenStr := str.Join(tokens, " ")

                row := sparkle(sf("  %-5d %-3s [#dim]%-5d[#.] %-3s %s", i, bpMarker, line, scope, tokenStr))
                rows = append(rows, row)

                plainRow := Strip(StripCC(row))
                if len(plainRow) > maxWidth {
                    maxWidth = len(plainRow)
                }
            }

            // Display rows with full background coverage
            for _, row := range rows {

                plainRow := Strip(StripCC(row))
                pad := maxWidth - len(plainRow)
                if pad < 0 {
                    pad = 0
                }

                pf("%s%s[#-]\n", row, str.Repeat(" ", pad))
            }

            // Footer line
            pf(str.Repeat("─", maxWidth) + "\n")

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
            currentFile := filename // getFileFromIFS(ifs)
            pf("[#fgreen]Current file: %s[#-]\n", currentFile)

        case "b", "breakpoints":

            pf("[#bold]Current breakpoints:[#-]\n")
            d.lock.RLock()
            for bpKey, cond := range d.breakpoints {
                bpIFS := uint32(bpKey >> 32)
                pc := int(bpKey & 0xffffffff)

                // Determine correct sourceBase for decoding
                // phrases := functionspaces[sourceBase]

                lineNum := -1
                if pc >= 0 && pc < len(phrases) {
                    lineNum = int(phrases[pc].SourceLine)
                }

                file := filename // getFileFromIFS(sourceBase)
                funcName, _ := numlookup.lmget(bpIFS)
                if cond == "" {
                    pf("  %s:%d (%s)\n", file, lineNum, funcName)
                } else {
                    pf("  %s:%d (%s) [#fyellow][if %s][#-]\n", file, lineNum, funcName, cond)
                }
            }
            d.lock.RUnlock()


        case "b+","ba":

            pf("Enter statement index (PC) for breakpoint: ")
            idxStr, _ := reader.ReadString('\n')
            idxStr = str.TrimSpace(idxStr)
            idx, err := strconv.Atoi(idxStr)
            if err != nil {
                pf("[#fred]Invalid PC index: %v[#.]\n", err)
                break
            }

            pf("Enter optional condition (or leave blank): ")
            cond, _ := reader.ReadString('\n')
            cond = str.TrimSpace(cond)

            // Validate condition if present
            if cond != "" {
                _, err := ev(p, ifs, cond)
                if err != nil {
                    pf("[#fred]Error in condition expression: %v[#.]\n", err)
                    break
                }
                pf("[#fyellow]⚠️  Warning: Conditional breakpoints may slow down execution.[#.]\n")
            }

            key := (uint64(ifs) << 32) | uint64(idx)
            debugger.lock.Lock()
            debugger.breakpoints[key] = cond
            debugger.lock.Unlock()
            pf("[#fgreen]Breakpoint added at PC index: %d (0x%x)[#.]\n", idx, key)


        case "b-","br":
            pf("Enter statement index (PC) to remove breakpoint: ")
            idxStr, _ := reader.ReadString('\n')
            idxStr = str.TrimSpace(idxStr)
            idx, err := strconv.Atoi(idxStr)
            if err != nil {
                pf("[#fred]Invalid PC index: %v[#.]\n", err)
                break
            }

            key := (uint64(ifs) << 32) | uint64(idx)
            debugger.lock.Lock()
            delete(debugger.breakpoints, key)
            debugger.lock.Unlock()
            pf("[#fgreen]Breakpoint removed at PC index: %d (0x%x)[#.]\n", idx, key)


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
            pf("[#fred]Exiting interpreter.[#-]\n\n")
            os.Exit(0)

        case "h", "help":
            pf(`
[#bold]Debugger commands:[#-]
  [#bold]c / continue[#-]       - Resume script execution.
  [#bold]s / step[#-]           - Execute next statement (step into).
  [#bold]n / next[#-]           - Execute next statement in current function (step over).
  [#bold]l / list[#-]           - Show current statement tokens.
  [#bold]ctx[#-]                - Set the list mode line spread context size.
  [#bold]v / vars[#-]           - Dump local variables (via stdlib dump()).
  [#bold]mvars[#-]              - Dump module/global-scope variables (via mdump()).
  [#bold]gvars[#-]              - Dump system variables (via gdump()).
  [#bold]sf / showf[#-]         - Show function definitions.
  [#bold]ss / shows[#-]         - Show struct definitions.
  [#bold]p / print <var>[#-]    - Show a single variable's value.
  [#bold]bt / where[#-]         - Show call chain backtrace.
  [#bold]fn / file[#-]          - Show current file name.
  [#bold]b / breakpoints[#-]    - List all breakpoints.
  [#bold]b+ / ba[#-]            - Add a breakpoint interactively.
  [#bold]b- / br[#-]            - Remove a breakpoint.
  [#bold]d / dis[#-]            - Token disassembly.
  [#bold]w / watch[#-]          - Add a variable to watch list.
  [#bold]uw / unwatch[#-]       - Remove a variable from watch list.
  [#bold]wl / watchlist[#-]     - Show watched variables.
  [#bold]e / eval[#-]           - Evaluate an expression in current scope.
  [#bold]src / source[#-]       - Source commands from a file.
  [#bold]ton / traceon[#-]      - Toggle line-by-line tracing on.
  [#bold]toff / traceoff[#-]    - Toggle line-by-line tracing off.
  [#bold]fs / functionspace[#-] - Show debug entry point.
  [#bold]cls[#-]                - Clear the screen.
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

