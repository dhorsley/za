package main

import (
	"reflect"
	"testing"
)

func Test_yyParserImpl_Lookahead(t *testing.T) {
	type fields struct {
		lval   yySymType
		stack  [yyInitialStackSize]yySymType
		char   int
		evalfs uint64
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &yyParserImpl{
				lval:   tt.fields.lval,
				stack:  tt.fields.stack,
				char:   tt.fields.char,
				evalfs: tt.fields.evalfs,
			}
			if got := p.Lookahead(); got != tt.want {
				t.Errorf("yyParserImpl.Lookahead() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestYyNewParser(t *testing.T) {
	tests := []struct {
		name string
		want yyParser
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := YyNewParser(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("YyNewParser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_yyTokname(t *testing.T) {
	type args struct {
		c int
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
			if got := yyTokname(tt.args.c); got != tt.want {
				t.Errorf("yyTokname() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_yyStatname(t *testing.T) {
	type args struct {
		s int
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
			if got := yyStatname(tt.args.s); got != tt.want {
				t.Errorf("yyStatname() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_yyErrorMessage(t *testing.T) {
	type args struct {
		state     int
		lookAhead int
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
			if got := yyErrorMessage(tt.args.state, tt.args.lookAhead); got != tt.want {
				t.Errorf("yyErrorMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_yylex1(t *testing.T) {
	type args struct {
		lex  yyLexer
		lval *yySymType
	}
	tests := []struct {
		name      string
		args      args
		wantChar  int
		wantToken int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotChar, gotToken := yylex1(tt.args.lex, tt.args.lval)
			if gotChar != tt.wantChar {
				t.Errorf("yylex1() gotChar = %v, want %v", gotChar, tt.wantChar)
			}
			if gotToken != tt.wantToken {
				t.Errorf("yylex1() gotToken = %v, want %v", gotToken, tt.wantToken)
			}
		})
	}
}

func Test_yyParse(t *testing.T) {
	type args struct {
		yylex  yyLexer
		evalfs uint64
	}
	tests := []struct {
		name  string
		args  args
		want  int
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := yyParse(tt.args.yylex, tt.args.evalfs)
			if got != tt.want {
				t.Errorf("yyParse() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("yyParse() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_yyParserImpl_Parse(t *testing.T) {
	type fields struct {
		lval   yySymType
		stack  [yyInitialStackSize]yySymType
		char   int
		evalfs uint64
	}
	type args struct {
		yylex  yyLexer
		evalfs uint64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
		want1  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yyrcvr := &yyParserImpl{
				lval:   tt.fields.lval,
				stack:  tt.fields.stack,
				char:   tt.fields.char,
				evalfs: tt.fields.evalfs,
			}
			got, got1 := yyrcvr.Parse(tt.args.yylex, tt.args.evalfs)
			if got != tt.want {
				t.Errorf("yyParserImpl.Parse() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("yyParserImpl.Parse() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
