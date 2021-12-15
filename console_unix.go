// +build !windows

package main

import (
    "bytes"
    term "github.com/pkg/term"
    "golang.org/x/sys/unix"
    str "strings"
    "time"
)


func setEcho(s bool) {
    if s {
        enableEcho()
    } else {
        disableEcho()
    }
}

func isatty() bool {
    _ , err := unix.IoctlGetTermios(0, ioctlReadTermios)
    return err==nil
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
    if tt!=nil {
        tt.Restore()
        tt.Close()
    }
}

// not on linux:
func GetWinInfo(fd int) (i int) {
    return -1
}

/// get keypresses, filtering out undesired until a valid match found
func wrappedGetCh(p int,disp bool) (i int) {

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
            if disp { pf("key : %#v\n",c) }
            if c != nil {
                switch {
                case bytes.Equal(c, []byte{2}):
                    k = 2 // ctrl-b
                case bytes.Equal(c, []byte{3}):
                    k = 3 // ctrl-c
                case bytes.Equal(c, []byte{4}):
                    k = 4 // ctrl-d
                case bytes.Equal(c, []byte{13}):
                    k = 13 // enter
                case bytes.Equal(c, []byte{0xc2, 0xa3}): // 194 163
                    k = 163
                case bytes.Equal(c, []byte{127}):
                    k = 127 // backspace
                case bytes.Equal(c, []byte{27,91,53,126}): // pgup
                    k = 15 // replaces Shift In (SI)
                case bytes.Equal(c, []byte{27,91,54,126}): // pgdown
                    k = 14 // replaces Shift Out (SO)
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
                case bytes.Equal(c, []byte{0x1B}): // ESCAPE
                    k = 27
                default:
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
var bigbytelist = make([]byte,6*4096)


// get a key press
func getch(timeo int) ( []byte, bool, bool, string ) {

    term.RawMode(tt)

    tt.SetOption(term.ReadTimeout(time.Duration(timeo) * time.Millisecond))
    numRead, err := tt.Read(bigbytelist)

    tt.Restore()

    // deal with mass input (pasting?)
    if numRead>6 {
        return []byte{0},false,true,string(bigbytelist[0:numRead])
    }

    // numRead can be up to 6 chars for special input stroke.

    if err != nil {
        // treat as timeout.. separate later, but timeout is buried in here
        return nil, true, false, ""
    }
    return bigbytelist[0:numRead], false, false, ""
}


// GetCursorPos()
// @note: don't use this if you can avoid it. better to track the cursor yourself
// than rely on this if you require even modest performance. reads the cursor
// position from the vt console itself using output commands. of course, speed is
// also externally dependant upon the vt emulation of the terminal software the
// program is executed within!

func GetCursorPos() (int,int) {

    if tt==nil {
        // return 0,0
        return -1,-1
    }

    buf:=make([]byte,15,15)
    var r,c int

    term.RawMode(tt)

    tt.Write([]byte("\033[6n"))

    n,_:=tt.Read(buf)

    if n>0 {
        endpos:=str.IndexByte(string(buf),'R')
        if endpos==-1 {
            r=-1; c=-1
        } else {
            op:=string(buf[2:endpos])
            parts:=str.Split(op,";")
            r,_=GetAsInt(parts[0])
            c,_=GetAsInt(parts[1])
        }
    }

    tt.Restore()

    return r,c

}


// GetSize returns the dimensions of the given terminal.
func GetSize(fd int) (int, int, error) {
    ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
    if err != nil {
        return -1, -1, err
    }
    return int(ws.Col), int(ws.Row), nil
}


