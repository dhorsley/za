package main

import "testing"

func Test_parse(t *testing.T) {
	type args struct {
		fs    string
		input string
		start int
	}
	tests := []struct {
		name        string
		args        args
		wantBadword bool
		wantEof     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBadword, gotEof := parse(tt.args.fs, tt.args.input, tt.args.start)
			if gotBadword != tt.wantBadword {
				t.Errorf("parse() gotBadword = %v, want %v", gotBadword, tt.wantBadword)
			}
			if gotEof != tt.wantEof {
				t.Errorf("parse() gotEof = %v, want %v", gotEof, tt.wantEof)
			}
		})
	}
}
