package main

//
// CONSTANTS
//

const MaxUint64 = ^uint64(0)

const MAX_LOOPS = 16
const DEFAULT_INIT_SIZE = 64   // start size of INIT'ed arrays

// maximum lib-net concurrent listener clients for http server
const MAX_CLIENTS = 800

const SPACE_CAP = 32000         // max user function instances ack(4,1) memoised uses ~16.5k
const CALL_CAP = 20             // calltable (open calls) start capacity. scales up.
const FUNC_CAP = 300            // max stdlib functions
const LOOP_START_CAP = 8        // max loops per function 
const VAR_CAP = 20              // max vars per function (scales up)
const FAIRY_CAP = 256           // max ansi mappings
const LIST_SIZE_CAP = 32        // initial list size on construction

const globalspace = uint64(0)   // global namespace

const promptStringStartup = "[#bgreen][#0]>[#-][##] "
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

// used by C_Endfor statement:
const (
	Opt_LoopStart int = iota
	Opt_LoopSet
	Opt_LoopIgnore
)

// used by Call() function. ENACT currently used by interactive mode.
const (
	MODE_CALL int = iota
	MODE_ENACT
	MODE_NEW
	MODE_STATIC
)

const (
	ACT_NONE int = iota
	ACT_INC
	ACT_DEC
)

const (
	IT_CHAR int = iota
	IT_LINE
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
)

const (
	Error int = iota
	EscapeSequence
	StringLiteral
	NumericLiteral
	Identifier
	Expression
	OptionalExpression
	Operator
	SingleComment
	MultiComment
	C_Plus
	C_Minus
	C_Divide
	C_Multiply
	C_Caret
	C_Pling
	C_Percent
	C_Semicolon
	LeftSBrace
	RightSBrace
	SYM_EQ
	SYM_LT
	SYM_LE
	SYM_GT
	SYM_GE
	SYM_NE
	SYM_AMP
    C_Comma
	C_Tilde
	C_Assign
	C_SetGlob
	C_Zero
	C_Inc
	C_Dec
	C_AssCommand
	C_LocalCommand
	C_RemoteCommand
	C_Init
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
	C_Showdef
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
	C_In
	C_Or
	C_Endwhen
    C_With
    C_Endwith
	C_Pane
	C_Doc
	C_Test
	C_Endtest
	C_Assert
	C_On
	EOL
	EOF // must remain at end of list
)
