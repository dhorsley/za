package main

//
// CONSTANTS
//

const MaxUint64 = ^uint64(0)
const MaxUint32 = ^uint32(0)

const MAX_LOOPS = 8
const DEFAULT_INIT_SIZE = 32   // start size of INIT'ed arrays

// maximum lib-net concurrent listener clients for http server
const MAX_CLIENTS = 800
const MAX_FUNCS = 10000         // max source funcs (not max instances)
const SPACE_CAP = 25000         // max user function instances ack(4,1) memoised uses ~16.5k
const CALL_CAP = 1000           // calltable (open calls) start capacity. scales up.
const FUNC_CAP = 300            // max stdlib functions
const LOOP_START_CAP = 8        // max loops per function 
const VAR_CAP = 8               // max vars per function (scales up)
const FAIRY_CAP = 64            // max ansi mappings
const LIST_SIZE_CAP = 16        // initial list size on construction
const WHEN_START_CAP = 2        // how many initial placeholders to create for WHEN...ENDWHEN meta info per func

const globalspace = uint32(0)   // global namespace

const promptStringStartup = "[#b4][#0]>>[#-][##] "
const promptContinuation  = "[#b6][#0]--[#-][##] "
const promptBashlike = "[#3]{@user}@{@hostname}[#-]:[#6]{@pwd}[#-] > "
const promptStringShort = "[#1]{@user}[#-] : [#invert]{@pwd}[#-] : "
const promptColour = "[#6]"
const recolour = "[#5][#i1]"

const default_WriteMode = 0644

const (
    WEB_PROXY int = iota
    WEB_REWRITE
    WEB_FUNCTION
    WEB_ERROR
    WEB_REDIRECT
)

    //  chainInfoRegistrants:
    //               0: Trap Handler
    //               1: Call Function
    //               2: User Defined Eval
    //               3: Module Definition
    //               4: Async Task
    //               5: Interactive Mode
    //               6: RHS Builder
    //               7: lib-net
    //               8: Main Routine
    //               9: Error Routines

const (
    ciTrap      uint8 = iota
    ciCall
    ciEval
    ciMod
    ciAsyn
    ciRepl
    ciRhsb
    ciLnet
    ciMain
    ciErr
)


// used by C_Endfor statement:
const (
	Opt_LoopStart uint8 = iota
	Opt_LoopSet
	Opt_LoopIgnore
)

// used by Call() function. ENACT currently used by interactive mode.
const (
	MODE_CALL uint8 = iota
	MODE_ENACT
	MODE_NEW
	MODE_STATIC
)

const (
	ACT_NONE uint8 = iota
	ACT_INC
	ACT_DEC
)

const (
	IT_LINE uint8 = iota
)

const (
	ERR_SYNTAX int = iota
	ERR_FATAL
	ERR_NARGS
	ERR_EXISTS
	ERR_EVAL
	ERR_NOBASH
	ERR_COPPER
	ERR_MODULE
	ERR_PACKAGE
	ERR_REQUIRE
	ERR_UNSUPPORTED
	ERR_ASSERT
    ERR_LEX int = 127
)

// IKind
const (
    knil uint8 = iota
    kbool
    kint
    kuint
    kfloat
    kstring
    kint64
)

const (
	Error uint8 = iota
	EOL
	EOF
	StringLiteral
	NumericLiteral
	Identifier
	Operator
	SingleComment
	O_Plus
	O_Minus
	O_Divide
	O_Multiply
	SYM_Caret
	SYM_Not
	O_Percent
	SYM_Semicolon
	O_Assign
	O_AssCommand
	LeftSBrace
	RightSBrace
    SYM_PLE
    SYM_MIE
    SYM_MUE
    SYM_DIE
    SYM_MOE
    LParen
    RParen
	SYM_EQ
	SYM_LT
	SYM_LE
	SYM_GT
	SYM_GE
	SYM_NE
    SYM_LAND
    SYM_LOR
    SYM_BAND
    SYM_BOR
    SYM_DOT
    SYM_PP
    SYM_MM
    SYM_POW
    SYM_RANGE
    SYM_LSHIFT
    SYM_RSHIFT
    SYM_COLON
    O_Comma
	SYM_Tilde
	SYM_ITilde
	SYM_FTilde
    O_Sqr
    O_Sqrt
    O_Query
    O_Filter
    O_Map
    START_STATEMENTS
    C_Var
	C_SetGlob
	C_Init
    C_In
	C_Pause
	C_Help
	C_Nop
	C_Hist
	C_Debug
	C_Require
	C_Exit
	C_Version
	C_Quiet
	C_Loud
	C_Unset
	C_Input
	C_Prompt
	C_Log
	C_Print
	C_Println
	C_Logging
	C_Cls
	C_At
	C_Define
	C_Enddef
	C_Return
    C_Async
	C_Lib
	C_Module
	C_Uses
	C_While
	C_Endwhile
	C_For
	C_Foreach
	C_Endfor
	C_Continue
	C_Break
	C_If
	C_Else
	C_Endif
	C_When
	C_Is
	C_Contains
    C_Has
	C_Or
	C_Endwhen
    C_With
    C_Endwith
    C_Struct
    C_Endstruct
    C_Showstruct
    C_Pane
	C_Doc
	C_Test
	C_Endtest
	C_Assert
	C_On
    C_To
    C_Step
    C_As
    C_Do
)
