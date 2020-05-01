package main

import (
	"bufio"
	_ "net/http/pprof"
	"os/exec"
	"regexp"
	str "strings"
	"testing"
)

var bsh *exec.Cmd

//var ident = make([]map[string]interface{},SPACE_CAP)
// var ident = make([][]Variable, SPACE_CAP)

func TestNewCoprocess(t *testing.T) {
	bsh, pi, po, pe = NewCoprocess("/bin/bash")
	t.Logf("bsh->%v\npi ->%v\npo ->%v\npe ->%v\n", bsh, pi, po, pe)
	if pi == nil || po == nil || pe == nil {
		t.Fatalf("expected all channels to be allocated.")
	}
	if bsh == nil {
	}
}

func TestGetBash(t *testing.T) {
	gb, err := GetBash("which bash")
	if gb == "" || err != nil {
		t.Fatalf("expected results from GetBash.")
	}
	if !str.HasSuffix(gb, "bash\n") {
		t.Logf("gb -> %v\n", gb)
		t.Fatalf("expected bash location.")
	}
}

func TestCopper(t *testing.T) {
	// ident[0] = make([]Variable, VAR_CAP)
	vcreatetable(0, VAR_CAP)
	vset(0, "mark_time", false)
	res, code := Copper("ls -l /", true)
	if code != 0 {
		t.Fatalf("expected zero in result code.")
	}
	rexp := regexp.MustCompile(" tmp\n")
	if !rexp.MatchString(res) {
		t.Fatalf("expected to find a /tmp directory.")
	}
	// deliberate fail:
	res, code = Copper("/i_dont_exist_hopefully", true)
	if code == 0 {
		t.Fatalf("expected non-zero in result code.")
	}
}

/* not important to test:

func Testdebug(t * testing.T) {
// level int,s string, va ...interface{}) {
}

func TestrestoreScreen(t * testing.T) {
// ) {
}

func TesttestStart(t * testing.T) {
// file string) {
}

func TesttestExit(t * testing.T) {
// ) {
}

func TestGetSize(t * testing.T) {
// fd int) (int,int,error) {
}

*/

func Test_fexists(t *testing.T) {
	type args struct {
		fp string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fexists(tt.args.fp); got != tt.want {
				t.Errorf("fexists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextCopper(t *testing.T) {
	type args struct {
		cmd string
		r   *bufio.Reader
	}
	tests := []struct {
		name    string
		args    args
		wantS   string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotS, err := NextCopper(tt.args.cmd, tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("NextCopper() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotS != tt.wantS {
				t.Errorf("NextCopper() = %v, want %v", gotS, tt.wantS)
			}
		})
	}
}

func Test_main(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			main()
		})
	}
}

func Test_debug(t *testing.T) {
	type args struct {
		level int
		s     string
		va    []interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			debug(tt.args.level, tt.args.s, tt.args.va...)
		})
	}
}

func Test_restoreScreen(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreScreen()
		})
	}
}

func Test_testStart(t *testing.T) {
	type args struct {
		file string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testStart(tt.args.file)
		})
	}
}

func Test_testExit(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testExit()
		})
	}
}

func TestGetSize(t *testing.T) {
	type args struct {
		fd int
	}
	tests := []struct {
		name    string
		args    args
		want    int
		want1   int
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetSize(tt.args.fd)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetSize() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetSize() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
