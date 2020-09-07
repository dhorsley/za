package main

import (
    "runtime"
)

func Evaluate(str string, evalfs uint64) (result interface{}, ef bool, err error) {

    // pf("gv-ev: entered with [efs:%v] %v\n",evalfs,str)

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
                pf("Dumping on (%v)\n",str)
				panic(r)
			}
			err = r.(error)
		}
	}()

    // lexer:=&Lexer{}
    // lexer.SetSource(str)
    lexer:=NewLexer(str)
	_,ef=YyNewParser().Parse(lexer, evalfs)

	return lexer.Result(), ef, err

}

