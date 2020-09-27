package main

//
// TYPES
//

import (
	"reflect"
    "time"
)

//
// this type is for holding a complete statement from statement to EOL/Semicolon
type Phrase struct {
	Tokens     []Token // each token found
	TokenCount int     // number of tokens generated for this phrase
    SourceLine int
	Original   string  // entire string, unmodified for spaces ( only for ON..DO, =| and | commands )
}

func (p Phrase) String() string {
	return p.Original
}

// ExpressionFunction can be called from within expressions.
type ExpressionFunction = func(evalfs uint64,args ...interface{}) (interface{}, error)


// za variable
type Variable struct {
    IName  string
    IKind  string
    IValue interface{}
    ITyped bool
}

// holds a Token which forms part of a Phrase.
type Token struct {
	tokType uint8       // token type from list in constants.go
    tokVal  interface{} // raw value storage
	tokText string      // the content of the token
}

func (t Token) String() string {
	return t.tokText
}

// holds the details of a function call.
type call_s struct {
	retvar      string      // the lhs var in the caller to be assigned back to
	fs          string      // the text name of the calling party
	caller      uint64      // the thing which made the call
	base        uint64      // the original functionspace location of the source
    callline    int         // from whence it came
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


// holds mappings for feature->category in the standard library.
type Feature struct {
	version  int    // for standard library and REQUIRE statement support.
	category string // for stdlib funcs() output splitting
}


// holds internal state for the WHEN command
type whenCarton struct {
	endLine   int         // where is the endWhen, so that we can break or skip to it
	dodefault bool        // set false when another clause has been active
	value     interface{} // the value should only ever be a string, int or float. IN only works with numbers.
}


// holds an expression to be evaluated and its result
type ExpressionCarton struct {
	assignVar string      // name of var to assign to
	text      string      // total expression
	result    interface{} // result of evaluation
    errVal    error
	assign    bool        // is this an assignment expression
	evalError bool        // did the evaluation succeed
}


//
// struct for loop internals
type s_loop struct {
	loopVar          string           // name of counter, populate ident[fs][loopVar] (float64) at start of each iteration. (C_Endfor)
	counter          int              // current position in loop
	condEnd          int              // terminating position value
    iterOverMap      *reflect.MapIter // stored iterator
	iterOverArray    interface{}      // stored value to iterate over from start expression
	loopType         uint8            // C_For, C_Foreach, C_While
	optNoUse         uint8            // for deciding if the local variable should reflect the loop counter
	repeatAction     uint8            // enum: ACT_NONE, ACT_INC, ACT_DEC
	repeatFrom       int              // line number
	repeatActionStep int              // size of repeatAction
	forEndPos        int              // ENDFOR location
	whileContinueAt  int              // if loop is WHILE, where is it's ENDWHILE
	repeatCond       []Token          // tested with wrappedEval() // used by while
}

// struct to support pseudo-windows in console
type Pane struct {
//	bg       string // currently unused, background colour
//	fg       string // currently unused, foreground colour
	col, row int    // top-left location in console
	w, h     int    // width and height
	boxed    string // border type
	title    string // window title
}

// stdlib types...

    type dirent struct {
        name    string
        size    int64
        mode    uint32
        mtime   time.Time
        isdir   bool
    }

    type token_result struct {
        tokens []string
        types  []string
    }

    type zainfo struct {
        version string
        name    string
        build   string
    }

    type web_info struct {
        result  string
        code    int
    }

