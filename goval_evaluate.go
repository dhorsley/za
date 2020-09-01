package main

import (
    "runtime"
    )

var	cacheParser yyParserImpl
var cacheLexer *Lexer

func Evaluate(str string, evalfs uint64) (result interface{}, ef bool, err error) {

    // pf("gv-ev: entered with %v\n",str)

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
                pf("Dumping on (%v)\n",str)
				panic(r)
			}
			err = r.(error)
		}
	}()

    lexer:=NewLexer(str)
    if lockSafety {
	    _,ef=YyNewParser().Parse(lexer, evalfs)
    } else {
        _,ef=cacheParser.Parse(lexer,evalfs)
    }

	return lexer.Result(), ef, err
}

