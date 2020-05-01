package main

import (
	"reflect"
	"testing"
)

func Test_nextToken(t *testing.T) {
	type args struct {
		input         string
		curLine       *int
		start         int
		previousToken int
	}
	tests := []struct {
		name       string
		args       args
		wantCarton Token
		wantEol    bool
		wantEof    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCarton, gotEol, gotEof := nextToken(tt.args.input, tt.args.curLine, tt.args.start, tt.args.previousToken)
			if !reflect.DeepEqual(gotCarton, tt.wantCarton) {
				t.Errorf("nextToken() gotCarton = %v, want %v", gotCarton, tt.wantCarton)
			}
			if gotEol != tt.wantEol {
				t.Errorf("nextToken() gotEol = %v, want %v", gotEol, tt.wantEol)
			}
			if gotEof != tt.wantEof {
				t.Errorf("nextToken() gotEof = %v, want %v", gotEof, tt.wantEof)
			}
		})
	}
}
