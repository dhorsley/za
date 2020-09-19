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
   // FileRef    uint64  // maybe not needed, file can be derived from fs/token context
    SourceLine int
}

func (p Phrase) String() string {
	return p.Text
}

// ExpressionFunction can be called from within expressions.
type ExpressionFunction = func(evalfs uint64,args ...interface{}) (interface{}, error)


// za variable
type Variable struct {
    IName  string
    IKind  string
    ITyped bool
    IValue interface{}
}

// holds a Token which forms part of a Phrase.
type Token struct {
    // name string
	tokType uint8         // token type from list in constants.go
    tokPos  int         // by character (from start of input)
    tokVal  interface{} // raw value storage
	//Line    int         // line in parsed string (1 based) of token
	//Col     int         // starting char position (from start of line) (not currently used)
	tokText string      // the content of the token
}

func (t Token) String() string {
	return t.tokText
}

// holds the details of a function call.
type call_s struct {
	caller      uint64      // the thing which made the call
	base        uint64      // the original functionspace location of the source
	fs          string      // the text name of the calling party
    callline    int         // from whence it came
	retvar      string      // the lhs var in the caller to be assigned back to
}

func (cs call_s) String() string {
	return sf("~{ fs %v - caller %v - return var %v }~", cs.fs, cs.caller, cs.retvar)
}


// chainInfo : used for passing debug info to error functions


type chainInfo struct {
    loc         uint64
    name        string
    line        int
    registrant  uint8
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
	dodefault bool        // set false when another clause has been active
	value     interface{} // the value should only ever be a string, int or float. IN only works with numbers.
}


//
// holds an expression to be evaluated and its result
type ExpressionCarton struct {
	assignVar string      // name of var to assign to
	text      string      // total expression
	result    interface{} // result of evaluation
	assign    bool        // is this an assignment expression
	evalError bool        // did the evaluation succeed
    errVal    error
}


//
// struct for loop internals
type s_loop struct {
	loopType         uint8            // C_For, C_Foreach, C_While
	loopVar          string           // name of counter, populate ident[fs][loopVar] (float64) at start of each iteration. (C_Endfor)
	counter          int              // current position in loop
	condEnd          int              // terminating position value
	repeatFrom       int              // line number
	repeatAction     int              // enum: ACT_NONE, ACT_INC, ACT_DEC
	repeatActionStep int              // size of repeatAction
	forEndPos        int              // ENDFOR location
	whileContinueAt  int              // if loop is WHILE, where is it's ENDWHILE
	optNoUse         int              // for deciding if the local variable should reflect the loop counter
    iterOverMap      *reflect.MapIter // stored iterator
	iterOverArray    interface{}      // stored value to iterate over from start expression
	repeatCond       []Token          // tested with wrappedEval() // used by while
}

// 
// struct to support pseudo-windows in console
type Pane struct {
//	bg       string // currently unused, background colour
//	fg       string // currently unused, foreground colour
	col, row int    // top-left location in console
	w, h     int    // width and height
	boxed    string // border type
	title    string // window title
}


