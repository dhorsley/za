// +build windows

package main

import (
    "bytes"
    "fmt"
    "syscall"
    "unsafe"
    "unicode/utf16"
    "time"
)

func setEcho(s bool) {

    var mode uint32
    pMode := &mode
    procGetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pMode)))

    var echoMode uint32
    echoMode = 4

    if s {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode | echoMode))
    } else {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode &^ echoMode))
    }

}

func isatty() bool {
    // windows version does not open a sub tty, uses cmd.exe instead
    return false
}

func disableEcho() {
    // does nothing
}

func enableEcho() {
    // does nothing
}

func GetCursorPos() (int,int) {
    tcol,trow,e:=GetRowCol(1)
    if e==nil {
        return trow,tcol
    }
    return -1,-1
}

func term_complete() {
    // does nothing
}


// for reference:
/*
ENABLE_PROCESSED_INPUT          = 0x0001
ENABLE_LINE_INPUT               = 0x0002
ENABLE_ECHO_INPUT               = 0x0004
ENABLE_WINDOW_INPUT             = 0x0008
ENABLE_MOUSE_INPUT              = 0x0010
ENABLE_INSERT_MODE              = 0x0020
ENABLE_QUICK_EDIT_MODE          = 0x0040
ENABLE_EXTENDED_FLAGS           = 0x0080
ENABLE_VIRTUAL_TERMINAL_INPUT   = 0x0200
*/


func getch(timeo int) (b []byte,timeout bool,pasted bool,paste_string string) {

    var mode uint32
    pMode := &mode
    procGetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pMode)))

    var vtMode, echoMode uint32
    echoMode        = 4
    vtMode          = 0x0200

    waitInput      := vtMode
    nowaitInput    := vtMode

    echo, _ := vget(0,&gident,"@echo")
    if echo.(bool) {
        waitInput += echoMode
        nowaitInput += echoMode
    }

    if timeo==0 {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr( waitInput ) )
    } else {
        procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr( nowaitInput ) )
    }

    line := make([]uint16, 3)
    pLine := &line[0]
    var n uint16

    c := make(chan []byte)
    closed:=false

    go func() {
        for ; ! closed ; {
            if timeo==0 {
                procReadConsole.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pLine)), uintptr(len(line)), uintptr(unsafe.Pointer(&n)))
                if n>0 && !closed {
                    c <- []byte(string(utf16.Decode(line[:n])))
                    break
                }
            } else {
                n=0
                procPeekConsoleInput.Call(uintptr(syscall.Stdin),uintptr(unsafe.Pointer(pLine)),uintptr(len(line)),uintptr(unsafe.Pointer(&n)))
                if n>0 {
                    procReadConsole.Call(uintptr(syscall.Stdin), uintptr(unsafe.Pointer(pLine)), uintptr(len(line)), uintptr(unsafe.Pointer(&n)))
                    closed=true
                    c <- []byte(string(utf16.Decode(line[:n])))
                    break
                }
                if timeout { break }
            }
        }
    }()

    if timeo>0 {
        dur := time.Duration(timeo) * time.Microsecond
        select {
        case b = <-c:
            procFlushConsoleInputBuffer.Call(uintptr(syscall.Stdin))
        case <-time.After(dur):
            timeout=true
        }
    } else {
        select {
        case b = <-c:
        }
    }

    procSetConsoleMode.Call(uintptr(syscall.Stdin), uintptr(mode))

    // we push a "never pasted" result back (false,"") from windows
    return b,timeout,false,""
}


var modkernel32 *syscall.LazyDLL
var procSetConsoleMode *syscall.LazyProc
var procFlushConsoleInputBuffer *syscall.LazyProc
var procPeekConsoleInput *syscall.LazyProc
var procReadConsole *syscall.LazyProc
var procGetConsoleMode *syscall.LazyProc

func setupDynamicCalls() {
    modkernel32 = syscall.NewLazyDLL("kernel32.dll")
    procSetConsoleMode = modkernel32.NewProc("SetConsoleMode")
    procFlushConsoleInputBuffer = modkernel32.NewProc("FlushConsoleInputBuffer")
    procPeekConsoleInput = modkernel32.NewProc("PeekConsoleInputW")
    procReadConsole = modkernel32.NewProc("ReadConsoleW")
    procGetConsoleMode = modkernel32.NewProc("GetConsoleMode")
}


/// get a keypress
func wrappedGetCh(p int,disp bool) (k int) {

    c,tout,_,_ := getch(p)

    if !tout {

        if c != nil {
            switch {
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31,0x3b,0x32,0x41}): // SHIFT-UP
                k = 211
            case bytes.Equal(c, []byte{0x1B, 0x5B, 0x31,0x3b,0x32,0x42}): // SHIFT-DOWN
                k = 210
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
            case bytes.Equal(c, []byte{3}):
                k = 3 // ctrl-c
            case bytes.Equal(c, []byte{4}):
                k = 4 // ctrl-d
            case bytes.Equal(c, []byte{27,91,53,126}): // pgup
                k = 15 // replaces Shift In (SI)
            case bytes.Equal(c, []byte{27,91,54,126}): // pgdown
                k = 14 // replaces Shift Out (SO)
            case bytes.Equal(c, []byte{0x01}): // ESCAPE
                k = 27
            case bytes.Equal(c, []byte{13}):
                k = 13 // enter
            case bytes.Equal(c, []byte{0xc2, 0xa3}):
                k = 163 // £ 
            case bytes.Equal(c, []byte{126}):
                k = 126 // DEL
            case bytes.Equal(c, []byte{127}):
                k = 127 // backspace
            default:
                if len(c) == 1 {
                    if c[0] > 31 {
                        k = int(c[0])
                    }
                }
            }
        }

    }

    return k
}

type (
    SHORT int16
    WORD  uint16

    SMALL_RECT struct {
        Left   SHORT
        Top    SHORT
        Right  SHORT
        Bottom SHORT
    }

    COORD struct {
        X SHORT
        Y SHORT
    }

    CONSOLE_SCREEN_BUFFER_INFO struct {
        Size              COORD
        CursorPosition    COORD
        Attributes        WORD
        Window            SMALL_RECT
        MaximumWindowSize COORD
    }
)

func checkError(r1, r2 uintptr, err error) error {
    // Windows APIs return non-zero to indicate success
    if r1 != 0 {
        return nil
    }

    // Return the error if provided, otherwise default to EINVAL
    if err != nil {
        return err
    }
    return syscall.EINVAL
}

func getStdHandle(stdhandle int) uintptr {
    handle, err := syscall.GetStdHandle(stdhandle)
    if err != nil {
        panic(fmt.Errorf("could not get standard io handle %d", stdhandle))
    }
    return uintptr(handle)
}

func GetConsoleScreenBufferInfo(handle uintptr) (*CONSOLE_SCREEN_BUFFER_INFO, error) {
    var info CONSOLE_SCREEN_BUFFER_INFO
    var kernel32DLL = syscall.NewLazyDLL("kernel32.dll")
    var getConsoleScreenBufferInfoProc = kernel32DLL.NewProc("GetConsoleScreenBufferInfo")
    if err := checkError(getConsoleScreenBufferInfoProc.Call(handle, uintptr(unsafe.Pointer(&info)), 0)); err != nil {
        return nil, err
    }
    return &info, nil
}

func GetWinInfo(fd int) (info *CONSOLE_SCREEN_BUFFER_INFO) {
    stdoutHandle := getStdHandle(syscall.STD_OUTPUT_HANDLE)
    info, _ = GetConsoleScreenBufferInfo(stdoutHandle)
    return info
}

func GetSize(fd int) (width, height int, err error) {

    stdoutHandle := getStdHandle(syscall.STD_OUTPUT_HANDLE)
    var info, e = GetConsoleScreenBufferInfo(stdoutHandle)

    if e != nil {
            return 0, 0, e
    }

    // we should be able to use Size.Y here, but get a nonsense
    // answer back most of the time. (probably to do with max
    // history size?)

    // so we calculate height based on the moving window size
    // in the history window instead.

    y:=int(info.Window.Bottom)-int(info.Window.Top)

    // return int(info.Size.X), int(info.Size.Y), nil
    return int(info.Size.X), y, nil

}

func GetRowCol(fd int) (int, int, error) {
    stdoutHandle := getStdHandle(syscall.STD_OUTPUT_HANDLE)
    var info, e = GetConsoleScreenBufferInfo(stdoutHandle)

    if e != nil {
            return 0, 0, e
    }
    return int(info.CursorPosition.X), int(info.CursorPosition.Y), nil
}


