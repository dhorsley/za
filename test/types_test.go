package main

import (
	"reflect"
	"testing"
)

func TestPhrase_String(t *testing.T) {
	type fields struct {
		Text       string
		Original   string
		TokenCount int
		Tokens     []Token
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Phrase{
				Text:       tt.fields.Text,
				Original:   tt.fields.Original,
				TokenCount: tt.fields.TokenCount,
				Tokens:     tt.fields.Tokens,
			}
			if got := p.String(); got != tt.want {
				t.Errorf("Phrase.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToken_String(t *testing.T) {
	type fields struct {
		name    string
		tokType int
		tokPos  int
		tokText string
		tokVal  interface{}
		Line    int
		Col     int
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := Token{
				name:    tt.fields.name,
				tokType: tt.fields.tokType,
				tokPos:  tt.fields.tokPos,
				tokText: tt.fields.tokText,
				tokVal:  tt.fields.tokVal,
				Line:    tt.fields.Line,
				Col:     tt.fields.Col,
			}
			if got := tr.String(); got != tt.want {
				t.Errorf("Token.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_call_s_String(t *testing.T) {
	type fields struct {
		fs      string
		caller  uint64
		base    uint64
		retvars []string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := call_s{
				fs:      tt.fields.fs,
				caller:  tt.fields.caller,
				base:    tt.fields.base,
				retvars: tt.fields.retvars,
			}
			if got := cs.String(); got != tt.want {
				t.Errorf("call_s.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_s_loop_String(t *testing.T) {
	type fields struct {
		loopVar          string
		loopType         int
		iterType         int
		repeatFrom       int
		repeatCond       ExpressionCarton
		repeatAction     int
		repeatActionStep int
		ecounter         int
		counter          int
		econdEnd         int
		condEnd          int
		forEndPos        int
		whileContinueAt  int
		iterOverMap      *reflect.MapIter
		iterOverString   interface{}
		iterOverArray    interface{}
		optNoUse         int
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := s_loop{
				loopVar:          tt.fields.loopVar,
				loopType:         tt.fields.loopType,
				iterType:         tt.fields.iterType,
				repeatFrom:       tt.fields.repeatFrom,
				repeatCond:       tt.fields.repeatCond,
				repeatAction:     tt.fields.repeatAction,
				repeatActionStep: tt.fields.repeatActionStep,
				ecounter:         tt.fields.ecounter,
				counter:          tt.fields.counter,
				econdEnd:         tt.fields.econdEnd,
				condEnd:          tt.fields.condEnd,
				forEndPos:        tt.fields.forEndPos,
				whileContinueAt:  tt.fields.whileContinueAt,
				iterOverMap:      tt.fields.iterOverMap,
				iterOverString:   tt.fields.iterOverString,
				iterOverArray:    tt.fields.iterOverArray,
				optNoUse:         tt.fields.optNoUse,
			}
			if got := l.String(); got != tt.want {
				t.Errorf("s_loop.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
