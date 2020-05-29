package main

//
// TYPES
//

import (
	"reflect"
)

//
// this type is for holding a complete statement from statement to EOL/Semicolon
type Phrase struct {
	Text       string  // entire tokenised string
	Original   string  // entire string, unmodified for spaces
	TokenCount int     // number of tokens generated for this phrase
	Tokens     []Token // each token found
}

func (p Phrase) String() string {
	return p.Text
}


//
// holds a Token which forms part of a Phrase.
type Token struct {
    name string
	tokType int         // token type from list in constants.go
    tokPos  int         // by character (from start of input)
	tokText string      // the content of the token
    tokVal  interface{} // raw value storage
	Line    int         // line in parsed string (1 based) of token
	Col     int         // starting char position (from start of line) (not currently used)
}

func (t Token) String() string {
	return t.tokText
}


//
// holds the details of a function call.
type call_s struct {
	fs      string   // the text name of the calling party
	caller  uint64   // the thing which made the call
	base    uint64   // the original functionspace location of the source
	retvar  string   // the lhs var in the caller to be assigned back to
}

func (cs call_s) String() string {
	return sf("~{ fs %v - caller %v - return var %v }~", cs.fs, cs.caller, cs.retvar)
}


//
// holds mappings for feature->category in the standard library.
type Feature struct {
	version  int    // for standard library and REQUIRE statement support.
	category string // for stdlib funcs() output splitting
}


//
// holds internal state for the WHEN command
type whenCarton struct {
	endLine   int         // where is the endWhen, so that we can break or skip to it
	value     interface{} // the value should only ever be a string, int or float. IN only works with numbers.
	// broken    bool        // might not use this. set when a BREAK has been encountered.
	dodefault bool        // set false when another clause has been active
}


//
// holds an expression to be evaluated and its result
type ExpressionCarton struct {
	text      string      // total expression
	assign    bool        // is this an assignment expression
	assignVar string      // name of var to assign to
	evalError bool        // did the evaluation succeed
	// evalCode  int         // error code returned on failure
	// reason    string      // failure reason
	result    interface{} // result of evaluation
}


//
// struct for loop internals
type s_loop struct {
	loopVar          string           // name of counter, populate ident[fs][loopVar] (float64) at start of each iteration. (C_Endfor)
	loopType         int              // C_For, C_Foreach, C_While
	iterType         int              // from enum IT_CHAR, IT_LINE
	repeatFrom       int              // line number
	repeatCond       ExpressionCarton // tested with ev() // used by while
	repeatAction     int              // enum: ACT_NONE, ACT_INC, ACT_DEC
	repeatActionStep int              // size of repeatAction
	ecounter         int              // current position in loop
	counter          int              // current position in loop
	econdEnd         int              // terminating position value
	condEnd          int              // terminating position value
	forEndPos        int              // ENDFOR location
	whileContinueAt  int              // if loop is WHILE, where is it's ENDWHILE
	iterOverMap      *reflect.MapIter // stored iterator
	iterOverString   interface{}      // stored value to iterate over from start expression
	iterOverArray    interface{}      // stored value to iterate over from start expression
	optNoUse         int              // for deciding if the local variable should reflect the loop counter
}

func (l s_loop) String() string {
	var op string = "" // output string
	switch l.loopType {
	case C_For:
		repActionList := [...]string{"NONE", "INCREMENT", "DECREMENT"}
		op = "~{ [#5]loop: FOR \n"
		// pick out: loopVar, repeatAction, repeatActionStep, counter, condEnd
		op += sf("    variable   -> %v\n", l.loopVar)
		op += sf("    counter    -> %v\n", l.counter)
		op += sf("    condition  -> %v\n", l.condEnd)
		op += sf("    repAction  -> %v\n", repActionList[l.repeatAction])
		op += sf("    repStep    -> %v\n", l.repeatActionStep)
	case C_Foreach:
		iterTypeList := [...]string{"CHARACTER", "LINE"}
		op = "~{ [#6]loop: FOREACH \n"
		// pick out: loopVar, iterType, ecounter, econdEnd
		op += sf("    variable   -> %v\n", l.loopVar)
		op += sf("    type       -> %v\n", iterTypeList[l.iterType])
		op += sf("    counter    -> %v\n", l.ecounter)
		op += sf("    condition  -> %v\n", l.econdEnd)
	case C_While:
		op = "~{ [#4]loop: WHILE \n"
		// pick out: repeatCond
		// op+=sf("    condition  -> %v\n",l.repeatCond.spaced)
		op += sf("    condition  -> %v\n", l.repeatCond.text)
	default:
		op = "~{ loop: unknown type "
	}
	op += "[#-]}~\n"
	return sparkle(op)
}


// 
// struct to support pseudo-windows in console
type Pane struct {
	bg       string // currently unused, background colour
	fg       string // currently unused, foreground colour
	col, row int    // top-left location in console
	w, h     int    // width and height
	boxed    string // border type
	title    string // window title
}


