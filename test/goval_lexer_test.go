package main

import (
	"go/scanner"
	"go/token"
	"reflect"
	"testing"
)

func TestNewLexer(t *testing.T) {
	type args struct {
		src string
	}
	tests := []struct {
		name string
		args args
		want *Lexer
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewLexer(tt.args.src); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewLexer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLexer_scan(t *testing.T) {
	type fields struct {
		scanner       scanner.Scanner
		result        interface{}
		nextTokenType int
		nextTokenInfo eToken
	}
	tests := []struct {
		name   string
		fields fields
		want   token.Pos
		want1  token.Token
		want2  string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lexer{
				scanner:       tt.fields.scanner,
				result:        tt.fields.result,
				nextTokenType: tt.fields.nextTokenType,
				nextTokenInfo: tt.fields.nextTokenInfo,
			}
			got, got1, got2 := l.scan()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Lexer.scan() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Lexer.scan() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("Lexer.scan() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestLexer_Lex(t *testing.T) {
	type fields struct {
		scanner       scanner.Scanner
		result        interface{}
		nextTokenType int
		nextTokenInfo eToken
	}
	type args struct {
		lval *yySymType
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lexer{
				scanner:       tt.fields.scanner,
				result:        tt.fields.result,
				nextTokenType: tt.fields.nextTokenType,
				nextTokenInfo: tt.fields.nextTokenInfo,
			}
			if got := l.Lex(tt.args.lval); got != tt.want {
				t.Errorf("Lexer.Lex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLexer_Error(t *testing.T) {
	type fields struct {
		scanner       scanner.Scanner
		result        interface{}
		nextTokenType int
		nextTokenInfo eToken
	}
	type args struct {
		e string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lexer{
				scanner:       tt.fields.scanner,
				result:        tt.fields.result,
				nextTokenType: tt.fields.nextTokenType,
				nextTokenInfo: tt.fields.nextTokenInfo,
			}
			l.Error(tt.args.e)
		})
	}
}

func TestLexer_Perrorf(t *testing.T) {
	type fields struct {
		scanner       scanner.Scanner
		result        interface{}
		nextTokenType int
		nextTokenInfo eToken
	}
	type args struct {
		pos    token.Pos
		format string
		a      []interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lexer{
				scanner:       tt.fields.scanner,
				result:        tt.fields.result,
				nextTokenType: tt.fields.nextTokenType,
				nextTokenInfo: tt.fields.nextTokenInfo,
			}
			l.Perrorf(tt.args.pos, tt.args.format, tt.args.a...)
		})
	}
}

func TestLexer_Result(t *testing.T) {
	type fields struct {
		scanner       scanner.Scanner
		result        interface{}
		nextTokenType int
		nextTokenInfo eToken
	}
	tests := []struct {
		name   string
		fields fields
		want   interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lexer{
				scanner:       tt.fields.scanner,
				result:        tt.fields.result,
				nextTokenType: tt.fields.nextTokenType,
				nextTokenInfo: tt.fields.nextTokenInfo,
			}
			if got := l.Result(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Lexer.Result() = %v, want %v", got, tt.want)
			}
		})
	}
}
