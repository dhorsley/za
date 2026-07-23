package main

import (
	"fmt"
	"os"
	"za/lexer"
)

var tokNames = lexer.TokNames

type lcstruct struct {
	carton Token
	tokPos int
	eol    bool
	eof    bool
	borpos int
}

func nextToken(input string, fs uint32, curLine *int16, start int) *lcstruct {
	res, err := lexer.NextToken(input, fs, curLine, start, bind_int)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(ERR_LEX)
	}
	return convertResult(res)
}

func convertResult(lr *lexer.Result) *lcstruct {
	return &lcstruct{
		carton: convertToken(lr.Tok),
		tokPos: lr.Pos,
		eol:    lr.Eol,
		eof:    lr.Eof,
		borpos: lr.Borpos,
	}
}

func convertToken(lt lexer.Token) Token {
	return Token{
		tokType:          lt.TokType,
		bindpos:          lt.Bindpos,
		tokText:          lt.TokText,
		tokVal:           lt.TokVal,
		la_else_distance: lt.LaElseDistance,
		la_end_distance:  lt.LaEndDistance,
		subtype:          lt.Subtype,
		bound:            lt.Bound,
		la_done:          lt.LaDone,
		la_has_else:      lt.LaHasElse,
	}
}
