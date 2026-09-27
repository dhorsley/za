//go:build !windows

package main

import (
    "bytes"
    term "github.com/pkg/term"
    "golang.org/x/sys/unix"
    "runtime"
    str "strings"
    "syscall"
    "time"
)

// tt is the keystroke input receiver. It is declared here (rather than main.go)
// so the github.com/pkg/term dependency is not imported into Windows builds,
// where upstream's term_windows.go does not compile.
var tt *term.Term // keystroke input receiver

// ttPollFd is a second, read-only fd to the same controlling terminal, used
// by getch to detect a "quiet gap" at the end of a volumed paste burst (a
// termios VTIME cannot express this: its timer only starts after the first
// byte, so a long gap would block forever).
var ttPollFd int = -1

// openConsoleInput opens the console/terminal for keystroke input.
func openConsoleInput() *term.Term {
    t, _ := term.Open("/dev/tty")
    if f, err := unix.Open("/dev/tty", unix.O_RDONLY|unix.O_NOCTTY|unix.O_CLOEXEC, 0); err == nil {
        ttPollFd = f
    }
    return t
}

func startRaw(timeo int) {
    if tt == nil {
        return
    }
    term.RawMode(tt)
    timeoutRaw(timeo)
}

func endRaw() {
    if tt == nil {
        return
    }
    tt.Restore()
}

func timeoutRaw(timeo int) {
    if tt == nil {
        return
    }
    tt.SetOption(term.ReadTimeout(time.Duration(timeo) * time.Millisecond))
}

func procKill(pid int) {
    syscall.Kill(pid, syscall.SIGINT)
}

func setEcho(s bool) {
    if s {
        enableEcho()
    } else {
        disableEcho()
    }
}

func isatty() bool {
    _, err := unix.IoctlGetTermios(0, ioctlReadTermios)
    return err == nil
}

func disableEcho() {
    termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
    if err == nil {
        newState := *termios
        newState.Lflag |= unix.ICANON | unix.ISIG
        newState.Iflag |= unix.ICRNL
        newState.Lflag &^= unix.ECHO
        unix.IoctlSetTermios(0, ioctlWriteTermios, &newState)
    }
}

func enableEcho() {
    termios, err := unix.IoctlGetTermios(0, ioctlReadTermios)
    if err == nil {
        newState := *termios
        newState.Lflag |= unix.ICANON | unix.ISIG
        newState.Iflag |= unix.ICRNL
        newState.Lflag |= unix.ECHO
        unix.IoctlSetTermios(0, ioctlWriteTermios, &newState)
    }
}

func term_complete() {
    if tt != nil {
        // disable_mouse()
        tt.Restore()
        tt.Close()
    }
}

// not on linux:
func GetWinInfo(fd int) (i int) {
    return -1
}

// / get keypresses, filtering out undesired until a valid match found
func wrappedGetCh(p int, disp bool) (i int) {

    if runtime.GOOS != "windows" {
        if tt == nil {
            return 27 // ESC - no tty available
        }
        startRaw(0)
        defer endRaw()
    }

    var keychan chan int
    keychan = make(chan int, 1)

    go func() {
        var k int
        for {
            c, tout, pasted, _ := getch(p)
            if tout {
                break
            }
            if pasted {
                break
            }
            if disp {
                pf("key : %#v\n", c)
            }
            if c != nil {
                switch {
                case bytes.Equal(c, []byte{2}):
                    k = 2 // ctrl-b
                case bytes.Equal(c, []byte{3}):
                    k = 3 // ctrl-c
                case bytes.Equal(c, []byte{12}):
                    k = 12 // ctrl-l
                case bytes.Equal(c, []byte{4}):
                    k = 4 // ctrl-d
                case bytes.Equal(c, []byte{13}):
                    k = 13 // enter
                case bytes.Equal(c, []byte{0xc2, 0xa3}): // 194 163
                    k = 163
                case bytes.Equal(c, []byte{127}):
                    k = 127 // backspace
                case bytes.Equal(c, []byte{27, 91, 53, 126}): // pgup
                    k = 15 // replaces Shift In (SI)
                case bytes.Equal(c, []byte{27, 91, 54, 126}): // pgdown
                    k = 14 // replaces Shift Out (SO)
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x48}): // HOME
                    k = 16
                case bytes.Equal(c, []byte{0x1B, 0x4F, 0x48}): // HOME (rxvt)
                    k = 16
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x7E}): // HOME
                    k = 16
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x46}): // END
                    k = 17
                case bytes.Equal(c, []byte{0x1B, 0x4F, 0x46}): // END (rxvt)
                    k = 17
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x34, 0x7E}): // END
                    k = 17
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3b, 0x32, 0x41}): // SHIFT-UP
                    k = 211
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3b, 0x32, 0x42}): // SHIFT-DOWN
                    k = 210
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3b, 0x32, 0x43}): // SHIFT-RIGHT
                    k = 209
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31, 0x3b, 0x32, 0x44}): // SHIFT-LEFT
                    k = 208
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x42}): // DOWN
                    k = 10
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x41}): // UP
                    k = 11
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x44}): // LEFT
                    k = 8
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x43}): // RIGHT
                    k = 9
                case bytes.Equal(c, []byte{0x09}): // TAB
                    k = 7
                case bytes.Equal(c, []byte{0x1B, 0x5B, 0x5A}): // SHIFT-TAB
                    k = 6
                case bytes.Equal(c, []byte{0x1B}): // ESCAPE
                    k = 27
                default:
                    // fmt.Printf("<%#v>",c)
                    if len(c) == 1 {
                        if c[0] > 31 {
                            k = int(c[0])
                        }
                    }
                }
            }
            if k != 0 {
                keychan <- k
                break
            }
        }
        keychan <- 0
    }()

    select {
    case i = <-keychan:
    }

    return i
}

func setupDynamicCalls() {
    // this is populated in windows version
}

// race condition, yes... but who arranges concurrent keyboard access?
var bigbytelist = make([]byte, 6*4096)

// upper bound for a single volume-detected paste burst
const maxBurstPaste = 1 << 20

// bytes split off the head of a read because a control sequence was glued
// to typed text (fast typing); returned by the next getch() call.
var pendingKey []byte

// how long getch() will wait for the continuation of a partial escape
// sequence before giving it to the key handler anyway
const escGraceMs = 35

var (
    bracketedPasteStart = []byte{0x1B, 0x5B, 0x32, 0x30, 0x30, 0x7E}
    bracketedPasteEnd   = []byte{0x1B, 0x5B, 0x32, 0x30, 0x31, 0x7E}
)

// completeEscEnd returns the index just past the complete escape sequence
// starting at b[0] (which must be ESC), or -1 when it is not finished yet.
// Handles ESC <final> (e.g. ESC f), SS3 ESC O <final> and CSI ESC [ ... <final>.
func completeEscEnd(b []byte) int {
    if len(b) == 0 || b[0] != 0x1B {
        return -1
    }
    i := 1
    if i < len(b) && (b[i] == 0x5B || b[i] == 0x4F) { // CSI or SS3
        i++
        for i < len(b) && (b[i] >= 0x30 && b[i] <= 0x3F || b[i] >= 0x20 && b[i] <= 0x2F) {
            i++ // parameter and intermediate bytes
        }
    }
    if i < len(b) && b[i] >= 0x40 && b[i] <= 0x7E {
        return i + 1 // final byte
    }
    return -1
}

// escGraceGather completes an escape sequence that arrived split across
// reads: it polls for continuation bytes within escGraceMs and returns the
// assembled bytes. Trailing bytes after a complete sequence are parked in
// pendingKey so a following keystroke is never swallowed.
func escGraceGather(acc []byte) []byte {
    start := time.Now()
    for time.Since(start) < time.Duration(escGraceMs)*time.Millisecond {
        if end := completeEscEnd(acc); end > 0 {
            if end < len(acc) {
                pendingKey = append(pendingKey, acc[end:]...)
                acc = acc[:end]
            }
            return acc
        }
        if ttPollFd < 0 {
            return acc
        }
        pollFd := []unix.PollFd{{Fd: int32(ttPollFd), Events: unix.POLLIN}}
        n, perr := unix.Poll(pollFd, 5)
        if perr != nil {
            return acc
        }
        if n == 0 {
            continue // still inside the grace window; wait for the continuation
        }
        num, e := tt.Read(bigbytelist)
        if e != nil || num == 0 {
            return acc
        }
        acc = append(acc, bigbytelist[:num]...)
    }
    return acc
}

// get a key press
func getch(timeo int) ([]byte, bool, bool, string) {

    if runtime.GOOS != "windows" {
        timeoutRaw(timeo)
    }

    if tt == nil {
        return nil, true, false, ""
    }

    // leftover text split from an escape sequence arrives first
    if len(pendingKey) > 0 {
        b := append([]byte{}, pendingKey...)
        pendingKey = pendingKey[:0]
        return b, false, false, ""
    }

    var numRead int
    var err error
    for numRead == 0 && err == nil {
        numRead, err = tt.Read(bigbytelist)
    }

    if err != nil {
        // treat as timeout.. separate later, but timeout is buried in here
        return nil, true, false, ""
    }

    data := bigbytelist[0:numRead]

    // Check for VTE bracketed paste mode first
    if bytes.HasPrefix(data, bracketedPasteStart) {
        // Start of bracketed paste - collect until end marker. The
        // triggering read may already contain the whole bracketed paste
        // (start+content+end in one shot); seed the collector with the bytes
        // after the start marker instead of discarding them.
        return collectBracketedPaste(data[6:])
    }

    // A control sequence glued to typed text (fast typing) must not be lost:
    // hand the text part to the editor now and let the ESC tail come back
    // reassembled via pendingKey on the next call. This also stops the
    // volume-based paste heuristic below from stripping away the sequence.
    if ei := bytes.IndexByte(data, 0x1B); ei > 0 {
        pendingKey = append(pendingKey, data[ei:]...)
        return data[:ei], false, false, ""
    }

    // A read starting with ESC is a control sequence (arrow, modified arrow,
    // function key), however long it is: it must reach the key handler, not
    // the paste path. Give fragmented delivery a short grace period first.
    if numRead > 0 && data[0] == 0x1B {
        return escGraceGather(append([]byte{}, data...)), false, false, ""
    }

    // Fall back to volume-based detection
    if numRead > 6 {
        // A single large paste often arrives split across multiple reads.
        // Gather the whole burst (until the stream goes quiet) so no part of
        // it is truncated and treated as typed input afterwards. A termios
        // VTIME gap can't detect "no more data" reliably, so poll the tty.
        acc := make([]byte, 0, numRead+1024)
        acc = append(acc, data...)
        pollFd := []unix.PollFd{{Fd: int32(ttPollFd), Events: unix.POLLIN}}
        for ttPollFd >= 0 && len(acc) < maxBurstPaste {
            n, perr := unix.Poll(pollFd, 50) // 50ms of silence => burst over
            if perr != nil || n == 0 {
                break
            }
            num, e := tt.Read(bigbytelist)
            if e != nil || num == 0 {
                break
            }
            acc = append(acc, bigbytelist[:num]...)
        }
        return []byte{0}, false, true, string(acc)
    }

    // numRead can be up to 6 chars for special input stroke.
    return data, false, false, ""
}

func collectBracketedPaste(initial []byte) ([]byte, bool, bool, string) {
    pasteBuffer := append([]byte{}, initial...)

    for {
        // Check for end of bracketed paste (the seed may already contain it)
        if bytes.HasSuffix(pasteBuffer, bracketedPasteEnd) {
            // Remove only the end marker; the start marker was already
            // consumed by getch
            pasteBuffer = pasteBuffer[:len(pasteBuffer)-len(bracketedPasteEnd)]
            return []byte{0}, false, true, string(pasteBuffer)
        }

        term.RawMode(tt)
        tt.SetOption(term.ReadTimeout(100 * time.Millisecond))
        numRead, err := tt.Read(bigbytelist)
        tt.Restore()

        if err != nil {
            break
        }

        pasteBuffer = append(pasteBuffer, bigbytelist[0:numRead]...)
    }

    // If we get here, something went wrong with bracketed paste
    return []byte{0}, false, true, string(pasteBuffer)
}

// GetCursorPos()
// @note: don't use this if you can avoid it. better to track the cursor yourself
// than rely on this if you require even modest performance. reads the cursor
// position from the vt console itself using output commands. of course, speed is
// also externally dependant upon the vt emulation of the terminal software the
// program is executed within!

func GetCursorPos() (int, int) {

    if tt == nil {
        // return 0,0
        return -1, -1
    }

    buf := make([]byte, 15, 15)
    var r, c int

    term.RawMode(tt)

    tt.Write([]byte("\033[6n"))

    n, _ := tt.Read(buf)

    if n > 0 {
        endpos := str.IndexByte(string(buf), 'R')
        if endpos == -1 {
            r = -1
            c = -1
        } else {
            op := string(buf[2:endpos])
            parts := str.Split(op, ";")
            r, _ = GetAsInt(parts[0])
            c, _ = GetAsInt(parts[1])
        }
    }

    tt.Restore()

    return r, c

}

// GetSize returns the dimensions of the given terminal.
func GetSize(fd int) (int, int, error) {
    ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
    if err != nil {
        return -1, -1, err
    }
    w := int(ws.Col)
    h := int(ws.Row)
    if w == 0 {
        w = -1
    }
    if h == 0 {
        h = -1
    }
    return w, h, nil
    // return int(ws.Col), int(ws.Row), nil
}

// handleCtrlZ sends SIGTSTP to suspend the process on Unix systems
func handleCtrlZ() {
    syscall.Kill(0, syscall.SIGTSTP)
}
