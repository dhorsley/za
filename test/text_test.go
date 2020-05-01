package main

import "testing"

func Test_lastCharSize(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastCharSize(tt.args.s); got != tt.want {
				t.Errorf("lastCharSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_pad(t *testing.T) {
	type args struct {
		s    string
		just int
		w    int
		fill string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pad(tt.args.s, tt.args.just, tt.args.w, tt.args.fill); got != tt.want {
				t.Errorf("pad() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_stripOuter(t *testing.T) {
	type args struct {
		s string
		c byte
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripOuter(tt.args.s, tt.args.c); got != tt.want {
				t.Errorf("stripOuter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_stripSingleQuotes(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripSingleQuotes(tt.args.s); got != tt.want {
				t.Errorf("stripSingleQuotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_stripDoubleQuotes(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripDoubleQuotes(tt.args.s); got != tt.want {
				t.Errorf("stripDoubleQuotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_stripOuterQuotes(t *testing.T) {
	type args struct {
		s        string
		maxdepth int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripOuterQuotes(tt.args.s, tt.args.maxdepth); got != tt.want {
				t.Errorf("stripOuterQuotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasOuterBraces(t *testing.T) {
	type args struct {
		s string
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
			if got := hasOuterBraces(tt.args.s); got != tt.want {
				t.Errorf("hasOuterBraces() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasOuter(t *testing.T) {
	type args struct {
		s string
		c byte
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
			if got := hasOuter(tt.args.s, tt.args.c); got != tt.want {
				t.Errorf("hasOuter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasOuterSingleQuotes(t *testing.T) {
	type args struct {
		s string
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
			if got := hasOuterSingleQuotes(tt.args.s); got != tt.want {
				t.Errorf("hasOuterSingleQuotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasOuterDoubleQuotes(t *testing.T) {
	type args struct {
		s string
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
			if got := hasOuterDoubleQuotes(tt.args.s); got != tt.want {
				t.Errorf("hasOuterDoubleQuotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_processString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processString(tt.args.s); got != tt.want {
				t.Errorf("processString() = %v, want %v", got, tt.want)
			}
		})
	}
}
