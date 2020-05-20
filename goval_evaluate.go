package main

import (
	"runtime"
)

func Evaluate(str string, evalfs uint64) (result interface{}, ef bool, err error) {

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
	_,ef=YyNewParser().Parse(lexer, evalfs)
	return lexer.Result(), ef, err
}
