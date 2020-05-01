package main

import (
	"reflect"
	"testing"
)

func Test_pf(t *testing.T) {
	type args struct {
		s  string
		va []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "pf.1",
            args: args{
                s   : "this %s %s %s",
                va  : []interface{}{"is","a","test  "},
            },
        },
	}
    prompt=true
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf(tt.args.s, tt.args.va...)
		})
	}
}

func Test_plog(t *testing.T) {
	type args struct {
		s  string
		va []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "plog.1",
            args: args{
                s   : "this %s %s %s",
                va  : []interface{}{"is","a","test  "},
            },
        },
	}
    vcreatetable(0, VAR_CAP)
    vset(0,"@silentlog",false)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plog(tt.args.s, tt.args.va...)
		})
	}
}

func Test_gpf(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "gpf.1",
            args: args{
                s   : "this is a test  ",
            },
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpf(tt.args.s)
		})
	}
}

func Test_spf(t *testing.T) {
	type args struct {
		ns uint64
		s  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "spf.1",
            args: args{
                ns  : 0,
                s   : "this is a test  ",
            },
            want: "this is a test  ",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spf(tt.args.ns, tt.args.s); got != tt.want {
				t.Errorf("spf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sparkle(t *testing.T) {
	type args struct {
		a string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "sparkle.1",
            args: args{
                a   : "{[#1]test[#-]  }",
            },
            want: "{\033[94mtest\033[0m  }",
        },
	}
    ansiMode=false
    setupAnsiPalette()
    ansiMode=true
    setupAnsiPalette()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sparkle(tt.args.a); got != tt.want {
				t.Errorf("sparkle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cls(t *testing.T) {
	tests := []struct {
		name string
	}{
        {
            name: "cls.1",
        },
	}
    vset(0,"@winterm",false)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cls()
		})
	}
}

func Test_setupAnsiPalette(t *testing.T) {
	tests := []struct {
		name string
	}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAnsiPalette()
		})
	}
}

func Test_paneLookup(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		wantRow int
		wantCol int
		wantW   int
		wantH   int
		wantErr bool
	}{
        {
            name: "panelookup.1",
            args: args{
                s: "global",
            },
            wantRow: 0,
            wantCol: 0,
            wantW: MW,
            wantH: MH,
            wantErr: false,
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRow, gotCol, gotW, gotH, err := paneLookup(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("paneLookup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotRow != tt.wantRow {
				t.Errorf("paneLookup() gotRow = %v, want %v", gotRow, tt.wantRow)
			}
			if gotCol != tt.wantCol {
				t.Errorf("paneLookup() gotCol = %v, want %v", gotCol, tt.wantCol)
			}
			if gotW != tt.wantW {
				t.Errorf("paneLookup() gotW = %v, want %v", gotW, tt.wantW)
			}
			if gotH != tt.wantH {
				t.Errorf("paneLookup() gotH = %v, want %v", gotH, tt.wantH)
			}
		})
	}
}

func TestStrip(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "strip.1",
            args: args{
                s: "\033[31mblah\033[42m",
            },
            want: "blah",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Strip(tt.args.s); got != tt.want {
				t.Errorf("Strip() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripCC(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "stripcc.1",
            args: args{
                s: "[#2]r[#6]o[#fbyellow]y[#4]g[#1]b[#3]i[#fbmagenta]v[#-]",
            },
            want: "roygbiv",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripCC(tt.args.s); got != tt.want {
				t.Errorf("StripCC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_displayedLen(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
        {
            name: "displayedlen.1",
            args: args{
                s: "[#2]r[#6]o[#fbyellow]y[#4]g[#1]b[#3]i[#fbmagenta]v[#-]",
            },
            want: 7,
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayedLen(tt.args.s); got != tt.want {
				t.Errorf("displayedLen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_absat(t *testing.T) {
	type args struct {
		row int
		col int
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "absat.1",
            args: args{ row: 0, col: 0, },
        },
        {
            name: "absat.2",
            args: args{ row: -1, col: -1, },
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absat(tt.args.row, tt.args.col)
		})
	}
}

func Test_at(t *testing.T) {
	type args struct {
		row int
		col int
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "at.1",
            args: args{ row: 0, col: 0, },
        },
        {
            name: "at.2",
            args: args{ row: -1, col: -1, },
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at(tt.args.row, tt.args.col)
		})
	}
}

func Test_getch(t *testing.T) {
	type args struct {
		timeo int
	}
	tests := []struct {
		name  string
		args  args
		want  []byte
		want1 bool
		want2 bool
		want3 string
	}{
        {
            name: "getch.1",
            args: args{
                timeo: 200,
            },
            want: nil,      // input char
            want1: true,    // timeout
            want2: false,   // multi-char input
            want3: "",      // multi-char buffer
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3 := getch(tt.args.timeo)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getch() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getch() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("getch() got2 = %v, want %v", got2, tt.want2)
			}
			if got3 != tt.want3 {
				t.Errorf("getch() got3 = %v, want %v", got3, tt.want3)
			}
		})
	}
}

func Test_clearToEOL(t *testing.T) {
	tests := []struct {
		name string
	}{
        {
            name: "cleartoeol.1",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearToEOL()
		})
	}
}

func Test_showCursor(t *testing.T) {
	tests := []struct {
		name string
	}{
        {
            name: "showcursor.1",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			showCursor()
		})
	}
}

func Test_hideCursor(t *testing.T) {
	tests := []struct {
		name string
	}{
        {
            name: "hidecursor.1",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hideCursor()
		})
	}
}

func Test_cursorX(t *testing.T) {
	type args struct {
		n int
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "cursorx.1",
            args: args{
                n: 1,
            },
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursorX(tt.args.n)
		})
	}
}

func Test_removeAllBefore(t *testing.T) {
	type args struct {
		s   string
		pos int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "removeallbefore.1",
            args: args{
                s: "startend",
                pos: 5,
            },
            want: "end",
        },
        {
            name: "removeallbefore.2",
            args: args{
                s: "startend",
                pos: 0,
            },
            want: "startend",
        },
        {
            name: "removeallbefore.3",
            args: args{
                s: "startend",
                pos: 8,
            },
            want: "",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeAllBefore(tt.args.s, tt.args.pos); got != tt.want {
				t.Errorf("removeAllBefore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_removeBefore(t *testing.T) {
	type args struct {
		s   string
		pos int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "removebefore.1",
            args: args{
                s: "startend",
                pos: 5,
            },
            want: "starend",
        },
        {
            name: "removebefore.2",
            args: args{
                s: "startend",
                pos: 0,
            },
            want: "startend",
        },
        {
            name: "removebefore.3",
            args: args{
                s: "startend",
                pos: 8,
            },
            want: "starten",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeBefore(tt.args.s, tt.args.pos); got != tt.want {
				t.Errorf("removeBefore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_insertBytesAt(t *testing.T) {
	type args struct {
		s   string
		pos int
		c   []byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "insertbytesat.1",
            args: args{
                s: "startend",
                pos: 5,
                c: []byte{'n','e','v','e','r'},
            },
            want: "startneverend",
        },
        {
            name: "insertbytesat.2",
            args: args{
                s: "startend",
                pos: 0,
                c: []byte{'n','e','v','e','r'},
            },
            want: "neverstartend",
        },
        {
            name: "insertbytesat.3",
            args: args{
                s: "startend",
                pos: 8,
                c: []byte{'n','e','v','e','r'},
            },
            want: "startendnever",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := insertBytesAt(tt.args.s, tt.args.pos, tt.args.c); got != tt.want {
				t.Errorf("insertBytesAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_insertAt(t *testing.T) {
	type args struct {
		s   string
		pos int
		c   byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "insertat.1",
            args: args{
                s: "startend",
                pos: 5,
                c: 'n',
            },
            want: "startnend",
        },
        {
            name: "insertat.2",
            args: args{
                s: "startend",
                pos: 0,
                c: 'n',
            },
            want: "nstartend",
        },
        {
            name: "insertat.3",
            args: args{
                s: "startend",
                pos: 8,
                c: 'n',
            },
            want: "startendn",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := insertAt(tt.args.s, tt.args.pos, tt.args.c); got != tt.want {
				t.Errorf("insertAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_insertWord(t *testing.T) {
	type args struct {
		s   string
		pos int
		w   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "insertword.1",
            args: args{
                s: "start phrase end",
                pos: 6,
                w: "new",
            },
            want: "start newphrase end",
        },
        {
            name: "insertword.2",
            args: args{
                s: "start phrase end",
                pos: 0,
                w: "new",
            },
            want: "newstart phrase end",
        },
        {
            name: "insertword.3",
            args: args{
                s: "start phrase end",
                pos: 16,
                w: "new",
            },
            want: "start phrase endnew",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := insertWord(tt.args.s, tt.args.pos, tt.args.w); got != tt.want {
				t.Errorf("insertWord() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deleteWord(t *testing.T) {
	type args struct {
		s   string
		pos int
	}
	tests := []struct {
		name string
		args args
		want1 string
        want2 int
	}{
        {
            name: "deleteword.1",
            args: args{
                s: "start phrase end",
                pos: 9,
            },
            want1: "start end",
            want2: 5,
        },
        {
            name: "deleteword.2",
            args: args{
                s: "start phrase end",
                pos: 0,
            },
            want1: "phrase end",
            want2: 0,
        },
        {
            name: "deleteword.3",
            args: args{
                s: "start phrase end",
                pos: 15,
            },
            want1: "start phrase",
            want2: 12,
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got1,got2 := deleteWord(tt.args.s, tt.args.pos)
            if got1 != tt.want1 {
				t.Errorf("deleteWord() = |%v|, want |%v|", got1, tt.want1)
			}
            if got2 != tt.want2 {
				t.Errorf("deleteWord() = |%v|, want |%v|", got2, tt.want2)
			}
		})
	}
}

/*
 * inappropriate for test: waits for manual input

func Test_wrappedGetCh(t *testing.T) {
	type args struct {
		p int
	}
	tests := []struct {
		name  string
		args  args
		wantI int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotI := wrappedGetCh(tt.args.p); gotI != tt.wantI {
				t.Errorf("wrappedGetCh() = %v, want %v", gotI, tt.wantI)
			}
		})
	}
}
 
*/

func Test_getWord(t *testing.T) {
	type args struct {
		s string
		c int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "getword.1",
            args: args{
                s: "this word here",
                c: 7,
            },
            want: "word",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getWord(tt.args.s, tt.args.c); got != tt.want {
				t.Errorf("getWord() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
 * inappropriate for test: waits for manual input, no timeout

func Test_getInput(t *testing.T) {
	type args struct {
		prompt     string
		pane       string
		row        int
		col        int
		pcol       string
		histEnable bool
		hintEnable bool
	}
	tests := []struct {
		name       string
		args       args
		wantS      string
		wantEof    bool
		wantBroken bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotS, gotEof, gotBroken := getInput(tt.args.prompt, tt.args.pane, tt.args.row, tt.args.col, tt.args.pcol, tt.args.histEnable, tt.args.hintEnable)
			if gotS != tt.wantS {
				t.Errorf("getInput() gotS = %v, want %v", gotS, tt.wantS)
			}
			if gotEof != tt.wantEof {
				t.Errorf("getInput() gotEof = %v, want %v", gotEof, tt.wantEof)
			}
			if gotBroken != tt.wantBroken {
				t.Errorf("getInput() gotBroken = %v, want %v", gotBroken, tt.wantBroken)
			}
		})
	}
}

*/


func Test_clearToEOPane(t *testing.T) {
	type args struct {
		row int
		col int
		va  []int
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "cleartoeopane.1",
            args: args{
                row: 1,
                col: 1,
                va: []int{},
            },
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearToEOPane(tt.args.row, tt.args.col, tt.args.va...)
		})
	}
}

func Test_paneBox(t *testing.T) {
	type args struct {
		c string
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "panebox.1",
            args: args{
                c: "global",
            },
        },
	}

    panes["global"] = Pane{row: 0, col: 0, h: MH, w: MW + 1}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paneBox(tt.args.c)
		})
	}
}

func Test_rep(t *testing.T) {
	type args struct {
		s string
		i int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
        {
            name: "rep.1",
            args: args{
                s: "rep",
                i: 4,
            },
            want: "reprepreprep",
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rep(tt.args.s, tt.args.i); got != tt.want {
				t.Errorf("rep() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_paneUnbox(t *testing.T) {
	type args struct {
		c string
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "paneunbox.1",
            args: args{
                c: "global",
            },
        },
	}

    panes["global"] = Pane{row: 0, col: 0, h: MH, w: MW + 1}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paneUnbox(tt.args.c)
		})
	}
}

func Test_setPane(t *testing.T) {
	type args struct {
		c string
	}
	tests := []struct {
		name string
		args args
	}{
        {
            name: "setpane.1",
            args: args{
                c: "global",
            },
        },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setPane(tt.args.c)
		})
	}
}



