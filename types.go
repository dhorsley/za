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
    SourceLine  int16
	TokenCount  int16   // number of tokens generated for this phrase
}

type BaseCode struct {
	Original    string  // entire string, unmodified for spaces
    borcmd      string  // issued command if SYM_BOR present
    HasFix      bool    // used by error recovery to see if handled by user FIX or system
}

type bc_block struct {
    Block       []byte
    compiled    bool
}

func (p BaseCode) String() string {
	return p.Original
}


type fa_s struct { // function args struct
    module  int
    args    []string
}


// "self" argument struct
type self_s struct {
    aware   bool
    ptr     *interface{}
}


// ExpressionFunction can be called from within expressions.
type ExpressionFunction = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (any, error)


// za variable
type Variable struct {
    IName       string
    IValue      any
    IKind       uint8
    ITyped      bool
    declared    bool
}

// holds a Token which forms part of a Phrase.
type Token struct {
    tokVal              any         // raw value storage
	tokText             string      // the content of the token
	tokType             uint8       // token type from list in constants.go
    subtype             uint8       // sub type of identifiers
    bound               bool
    bindpos             uint64
    la_else_distance    int16       // look ahead markers
    la_end_distance     int16
    la_has_else         bool
    la_done             bool
}

func (t Token) String() string {
	return t.tokText
}


// holds the details of a function call.
type call_s struct {
	caller      uint32      // the thing which made the call
	base        uint32      // the original functionspace location of the source
    gcShyness   uint32      // how many turns of the allocator before final disposal
    prepared    bool        // some fields pre-filled by caller
    gc          bool        // marked by Call() when disposable
    disposable  bool
	retvals     any         // returned values from the call
	fs          string      // the text name of the calling party
}

func (cs call_s) String() string {
	return sf("~{ fs %v - caller %v }~", cs.fs, cs.caller)
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

/*
// holds state about a while loop
type WhileMarker struct {
    pc          int16
    enddistance int
}
*/

// holds internal state for the WHEN command
type whenCarton struct {
	endLine   int16       // where is the endWhen, so that we can break or skip to it
	performed bool        // set to true by the matching clause
	dodefault bool        // set false when another clause has been active
	value     any         // the value should only ever be a string, int or float. IN only works with numbers.
}


// holds an expression to be evaluated and its result
type ExpressionCarton struct {
	assignVar string      // name of var to assign to
	text      string      // total expression
	result    any         // result of evaluation
    errVal    error
    assignPos int
	assign    bool        // is this an assignment expression
	evalError bool        // did the evaluation succeed
}

// struct for enum members
type enum_s struct {
    members   map[string]any
    ordered   []string
    namespace string
}


// struct for loop internals
type s_loop struct {
	loopVar          string           // name of counter
	loopVarBinding   uint64           // name binding lookup from loop name token
	counter          int              // current position in loop
	condEnd          int              // terminating position value
	forEndPos        int16            // ENDFOR location (statement number in functionspace)
	repeatFrom       int16            // line number to restart block from
	loopType         uint8            // C_For, C_Foreach, C_While
	optNoUse         uint8            // for deciding if the local variable should reflect the loop counter
	whileContinueAt  int16            // if loop is WHILE, where is it's ENDWHILE
    iterOverMap      *reflect.MapIter // stored iterator
	iterOverArray    any              // stored value to iterate over from start expression
	repeatCond       []Token          // tested with wrappedEval() // used by while + custom for conditions
	repeatActionStep int              // size of repeatAction
	repeatAction     uint8            // enum: ACT_NONE, ACT_INC, ACT_DEC
    repeatAmendment  []Token          // used by custom FOR conditions
    repeatCustom     bool             // FOR loop with custom conditions
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
        mode    int // from uint32
        mtime   int64
        is_dir  bool
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

    type alloc_info struct {
        id      int
        name    string
        size    int
    }

    type web_info struct {
        result  string
        code    int
    }

