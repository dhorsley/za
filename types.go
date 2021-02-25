package main

//
// TYPES
//

import (
	"reflect"
)

// this type is for holding a complete line from statement to EOL/Semicolon
type Phrase struct {
	Tokens      []Token // each token found
	Original    string  // entire string, unmodified for spaces
    pairLA      int16   // distance to next pc of end matching pair (0 is uncached)
    SourceLine  int16
	TokenCount  int16   // number of tokens generated for this phrase
}

func (p Phrase) String() string {
	return p.Original
}


type fa_s struct { // function args struct
    module  int
    args    []string
}


// ExpressionFunction can be called from within expressions.
type ExpressionFunction = func(evalfs uint32,args ...interface{}) (interface{}, error)


// za variable
type Variable struct {
    IName       string
    IValue      interface{}
    IKind       uint8
    ITyped      bool
    declared    bool
    // changed     bool
}

// holds a Token which forms part of a Phrase.
type Token struct {
    tokVal  interface{} // raw value storage
	tokText string      // the content of the token
    offset  uint16      // position in ident for this identifier
	tokType uint8       // token type from list in constants.go
}

func (t Token) String() string {
	return t.tokText
}


// holds the details of a function call.
type call_s struct {
	retvar      string      // the lhs var in the caller to be assigned back to
	fs          string      // the text name of the calling party
	caller      uint32      // the thing which made the call
	base        uint32      // the original functionspace location of the source
    callline    int16       // from whence it came
}

func (cs call_s) String() string {
	return sf("~{ fs %v - caller %v - return var %v }~", cs.fs, cs.caller, cs.retvar)
}

type Funcdef struct {
    name    string
    module  string
    fs      uint32
    // add extra fields here later. could maybe move fargs in
}

// chainInfo : used for passing debug info to error functions
type chainInfo struct {
    loc         uint32
    name        string
    line        int16
    registrant  uint8
}


// holds mappings for feature->category in the standard library.
type Feature struct {
	version  int    // for standard library and REQUIRE statement support.
	category string // for stdlib funcs() output splitting
}

// holds state about a while loop
type WhileMarker struct {
    pc          int16
    enddistance int
}

// holds internal state for the WHEN command
type whenCarton struct {
	endLine   int16       // where is the endWhen, so that we can break or skip to it
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

// struct for enum members
type enum_s struct {
    members   map[string]interface{}
    ordered   []string
}


// @todo: this need splitting up for several reasons. primarily though because of the size:
// struct for loop internals
type s_loop struct {
	loopVar          string           // name of counter
    varoffset        uint16           // position of varname in vmap array.
    varkeyoffset     uint16           // position in vmap that key_* will be placed.
	forEndPos        int16            // ENDFOR location (statement number in functionspace)
	loopType         uint8            // C_For, C_Foreach, C_While
	optNoUse         uint8            // for deciding if the local variable should reflect the loop counter
	counter          int              // current position in loop
	condEnd          int              // terminating position value
	iterOverArray    interface{}      // stored value to iterate over from start expression
                                      // one day, we'll just add a reference here ^^ instead! maybe
	whileContinueAt  int16            // if loop is WHILE, where is it's ENDWHILE
	repeatFrom       int16            // line number to restart block from
	repeatAction     uint8            // enum: ACT_NONE, ACT_INC, ACT_DEC
	repeatActionStep int              // size of repeatAction
	repeatCond       []Token          // tested with wrappedEval() // used by while
    iterOverMap      *reflect.MapIter // stored iterator
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
        mtime   int64
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

