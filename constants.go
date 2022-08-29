package main

//
// CONSTANTS
//

const MAX_LOOPS = 8

const identInitialSize = 8      // initial ident size on creation
const identGrowthSize = 8       // how many extra spaces to add when ident needs to grow
const gnfsModulus = 48000       // used by calltable to set max size, mainly impacts recursion

const MAX_CLIENTS = 800         // maximum lib-net concurrent listener clients for http server

const SPACE_CAP = gnfsModulus   // initial instance and source functions cap
const CALL_CAP = 1000           // calltable (open calls) start capacity. scales up.
const FUNC_CAP = 300            // stdlib functions storage space, starting point.
const FAIRY_CAP = 64            // max ansi mappings
const LIST_SIZE_CAP = 16        // initial list size on construction
const WHEN_CAP = 8              // how many placeholders to create for WHEN...ENDWHEN meta info per func
                                // ... this is currently only bounds checked in actor.go

const promptStringStartup = "[#b4][#0]>>[#-][##] "
const promptContinuation  = "[#b6][#0]--[#-][##] "
const promptBashlike = "[#3]{@user}@{@hostname}[#-]:[#6]{@cwd}[#-] > "
const promptStringShort = "[#1]{@user}[#-] : [#invert]{@cwd}[#-] : "
const defaultPromptColour = "[#6]"
const recolour = "[#5][#i1]"

const default_WriteMode = 0644

const (
    WEB_PROXY int = iota
    WEB_REWRITE
    WEB_FUNCTION
    WEB_ERROR
    WEB_REDIRECT
)

const (
    HELP_UNKNOWN int=iota
    HELP_FUNC
    HELP_KEYWORD
    HELP_DIRENT
)

const (
    S3_PT_NONE uint=iota
    S3_PT_SINGLE
    S3_PT_MULTI
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

// used by Call() function. MODE_STATIC currently used by interactive mode.
const (
	MODE_CALL uint8 = iota
	MODE_NEW                // instantiate and execute named function
    MODE_STATIC             // execute named function, from start, without reinit'ing local variable storage
)

// FOR loop counter direction:
const (
	ACT_NONE uint8 = iota
	ACT_INC
	ACT_DEC
)

// FOREACH string loop type. by-char deprecated.
const (
	IT_LINE uint8 = iota    // by line 
)


// identifier subtypes
const (
    subtypeNone = iota
    subtypeConst
    subtypeStandard
    subtypeUser
)

var subtypeNames = [...]string{"None","Constant","StdLib","UserFunc"}

// fatal error exit codes
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
    ERR_FILE
    ERR_LEX int = 127
)

// IKind, used by VAR
const (
    knil uint8 = iota
    kbool
    kbyte
    kint
    kint64
    kuint
    kuint64
    kfloat
    kbigi
    kbigf
    kstring
    kany
    ksbool
    ksint
    ksint64
    ksuint
    ksuint64
    ksfloat
    ksbigi
    ksbigf
    ksstring
    kmap
    ksany
    ksbyte
)


// Lexeme values
//  a few of these are unused now and some should probably be renamed.
//  they should be checked and tidied next time there is any change to be done here.
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
    O_AssOutCommand
	LeftSBrace
	RightSBrace
	LeftCBrace
	RightCBrace
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
    SYM_BSLASH
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
    O_InFile
    O_OutFile
    O_Ref
    O_Mut
    O_Slc
    O_Suc
    O_Sst
    O_Slt
    O_Srt
    O_Pb
    O_Pa
    O_Pn
    O_Pe
    O_Pp
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
    C_Showdef
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
    C_Enum
    Block
    AsyncBlock
    ResultBlock
    T_Number
    T_Nil
    T_Bool
    T_Int
    T_Uint
    T_Float
    T_Bigi
    T_Bigf
    T_String
    T_Map
    T_Array
    T_Any
    END_STATEMENTS
)


