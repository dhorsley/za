package main

import "za/lexer"

//
// CONSTANTS
//

const MAX_LOOPS = 16

const identInitialSize = 4       // initial ident size on creation
const identGrowthSize = 4        // how many extra spaces to add when ident needs to grow
const gnfsModulus = 48000        // used by calltable to set max size, mainly impacts recursion
const globseq_disposal_freq = 64 // sets the number of call allocations per calltable cleanup operation
const MAX_CLIENTS = 800          // maximum lib-net concurrent listener clients for http server

const SPACE_CAP = gnfsModulus // initial instance and source functions cap
const CALL_CAP = 200          // calltable (open calls) start capacity. scales up.
const FUNC_CAP = 300          // stdlib functions storage space, starting point.
const FAIRY_CAP = 64          // max ansi mappings
const LIST_SIZE_CAP = 16      // initial list size on construction
const CASE_CAP = 8            // how many placeholders to create for CASE...ENDCASE meta info per func
// ... this is currently only bounds checked in actor.go
const appGrowthFactor float64 = 1.5 // new slice capacity growth factor in lib-list:append()

const promptStringStartup = "[#b4][#0]>>[#-][##] "
const promptContinuation = "[#b6][#0]--[#-][##] "
const promptBashlike = "[#3]{@user}@{@hostname}[#-]:[#6]{@cwd}[#-] > "
const promptStringShort = "[#1]{@user}[#-] : [#invert]{@cwd}[#-] : "
const defaultPromptColour = "[#6]"
const recolour = "[#5][#i1]"

const default_WriteMode = 0644

const singularTolerance = 1e-12


const (
    WEB_PROXY int = iota
    WEB_REWRITE
    WEB_FUNCTION
    WEB_ERROR
    WEB_REDIRECT
)

const (
    HELP_UNKNOWN int = iota
    HELP_FUNC
    HELP_KEYWORD
    HELP_DIRENT
)

const (
    S3_PT_NONE uint = iota
    S3_PT_SINGLE
    S3_PT_MULTI
)

const (
    LT_FOR uint8 = iota
    LT_FOREACH
    LT_WHILE
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
//              10: FFI Callback

const (
    ciTrap uint8 = iota
    ciCall
    ciEval
    ciMod
    ciAsyn
    ciRepl
    ciRhsb
    ciLnet
    ciMain
    ciErr
    ciCallback
)

// used by C_Endfor statement:
const (
    Opt_LoopStart uint8 = iota
    Opt_LoopSet
    Opt_LoopIgnore
)

// used by Call() function. MODE_STATIC currently used by interactive mode.
const (
    MODE_CALL   uint8 = iota
    MODE_NEW          // instantiate and execute named function
    MODE_STATIC       // execute named function, from start, without reinit'ing local variable storage
    MODE_TRY          // execute try block sharing parent variable scope
)

// FOR loop counter direction:
const (
    ACT_NONE uint8 = iota
    ACT_INC
    ACT_DEC
)

// FOREACH string loop type. by-char deprecated.
const (
    IT_LINE uint8 = iota // by line
)

// identifier subtypes
const (
    subtypeNone     = lexer.SubtypeNone
    subtypeConst    = lexer.SubtypeConst
    subtypeStandard = lexer.SubtypeStandard
    subtypeUser     = lexer.SubtypeUser
    subtypeCUser    = lexer.SubtypeCUser
)

var subtypeNames = [...]string{"None", "Constant", "StdLib", "UserFunc"}

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
    ERR_EXCEPTION
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
    kspointer // for arrays of pointer types
    kdynamic // for dynamically constructed multi-dimensional types
    kpointer // for pointer types
    koutparam // for output parameters with unknown type (determined by FFI call)
)

type TokenType = int64

// Lexeme values
const (
    Error = lexer.Error
    EOL = lexer.EOL
    EOF = lexer.EOF
    StringLiteral = lexer.StringLiteral
    NumericLiteral = lexer.NumericLiteral
    Identifier = lexer.Identifier
    Operator = lexer.Operator
    SingleComment = lexer.SingleComment
    O_Plus = lexer.O_Plus
    O_Minus = lexer.O_Minus
    O_Divide = lexer.O_Divide
    O_Multiply = lexer.O_Multiply
    SYM_Caret = lexer.SYM_Caret
    SYM_Not = lexer.SYM_Not
    O_Percent = lexer.O_Percent
    SYM_Semicolon = lexer.SYM_Semicolon
    O_Assign = lexer.O_Assign
    O_AssCommand = lexer.O_AssCommand
    O_AssOutCommand = lexer.O_AssOutCommand
    LeftSBrace = lexer.LeftSBrace
    RightSBrace = lexer.RightSBrace
    LeftCBrace = lexer.LeftCBrace
    RightCBrace = lexer.RightCBrace
    SYM_PLE = lexer.SYM_PLE
    SYM_MIE = lexer.SYM_MIE
    SYM_MUE = lexer.SYM_MUE
    SYM_DIE = lexer.SYM_DIE
    SYM_MOE = lexer.SYM_MOE
    LParen = lexer.LParen
    RParen = lexer.RParen
    SYM_EQ = lexer.SYM_EQ
    SYM_LT = lexer.SYM_LT
    SYM_LE = lexer.SYM_LE
    SYM_GT = lexer.SYM_GT
    SYM_GE = lexer.SYM_GE
    SYM_NE = lexer.SYM_NE
    SYM_LAND = lexer.SYM_LAND
    SYM_LOR = lexer.SYM_LOR
    SYM_BAND = lexer.SYM_BAND
    SYM_BOR = lexer.SYM_BOR
    SYM_BSLASH = lexer.SYM_BSLASH
    SYM_DOT = lexer.SYM_DOT
    SYM_PP = lexer.SYM_PP
    SYM_MM = lexer.SYM_MM
    SYM_POW = lexer.SYM_POW
    SYM_RANGE = lexer.SYM_RANGE
    SYM_LSHIFT = lexer.SYM_LSHIFT
    SYM_RSHIFT = lexer.SYM_RSHIFT
    SYM_COLON = lexer.SYM_COLON
    SYM_DoubleColon = lexer.SYM_DoubleColon
    O_Comma = lexer.O_Comma
    SYM_Tilde = lexer.SYM_Tilde
    SYM_ITilde = lexer.SYM_ITilde
    SYM_FTilde = lexer.SYM_FTilde
    O_Sqr = lexer.O_Sqr
    O_Sqrt = lexer.O_Sqrt
    O_Query = lexer.O_Query
    O_Try = lexer.O_Try
    O_Filter = lexer.O_Filter
    O_Map = lexer.O_Map
    O_InFile = lexer.O_InFile
    O_OutFile = lexer.O_OutFile
    O_Ref = lexer.O_Ref
    O_Mut = lexer.O_Mut
    O_Slc = lexer.O_Slc
    O_Suc = lexer.O_Suc
    O_Sst = lexer.O_Sst
    O_Slt = lexer.O_Slt
    O_Srt = lexer.O_Srt
    O_Pb = lexer.O_Pb
    O_Pa = lexer.O_Pa
    O_Pn = lexer.O_Pn
    O_Pe = lexer.O_Pe
    O_Pp = lexer.O_Pp
    START_STATEMENTS = lexer.START_STATEMENTS
    C_Var = lexer.C_Var
    C_SetGlob = lexer.C_SetGlob
    C_Init = lexer.C_Init
    C_In = lexer.C_In
    C_Pause = lexer.C_Pause
    C_Help = lexer.C_Help
    C_Nop = lexer.C_Nop
    C_Hist = lexer.C_Hist
    C_Debug = lexer.C_Debug
    C_Require = lexer.C_Require
    C_Exit = lexer.C_Exit
    C_Version = lexer.C_Version
    C_Quiet = lexer.C_Quiet
    C_Loud = lexer.C_Loud
    C_Unset = lexer.C_Unset
    C_Input = lexer.C_Input
    C_Prompt = lexer.C_Prompt
    C_Log = lexer.C_Log
    C_Print = lexer.C_Print
    C_Println = lexer.C_Println
    C_Logging = lexer.C_Logging
    C_Cls = lexer.C_Cls
    C_At = lexer.C_At
    C_Define = lexer.C_Define
    C_Showdef = lexer.C_Showdef
    C_Enddef = lexer.C_Enddef
    C_Return = lexer.C_Return
    C_Async = lexer.C_Async
    C_Lib = lexer.C_Lib
    C_Module = lexer.C_Module
    C_Namespace = lexer.C_Namespace
    C_Use = lexer.C_Use
    C_Uses = lexer.C_Uses
    C_While = lexer.C_While
    C_Endwhile = lexer.C_Endwhile
    C_For = lexer.C_For
    C_Foreach = lexer.C_Foreach
    C_Endfor = lexer.C_Endfor
    C_Continue = lexer.C_Continue
    C_Break = lexer.C_Break
    C_If = lexer.C_If
    C_Else = lexer.C_Else
    C_Endif = lexer.C_Endif
    C_Case = lexer.C_Case
    C_Is = lexer.C_Is
    C_Contains = lexer.C_Contains
    C_Has = lexer.C_Has
    C_Or = lexer.C_Or
    C_Endcase = lexer.C_Endcase
    C_With = lexer.C_With
    C_Endwith = lexer.C_Endwith
    C_Struct = lexer.C_Struct
    C_Endstruct = lexer.C_Endstruct
    C_Showstruct = lexer.C_Showstruct
    C_Pane = lexer.C_Pane
    C_Doc = lexer.C_Doc
    C_Test = lexer.C_Test
    C_Endtest = lexer.C_Endtest
    C_Assert = lexer.C_Assert
    C_On = lexer.C_On
    C_To = lexer.C_To
    C_Step = lexer.C_Step
    C_As = lexer.C_As
    C_Do = lexer.C_Do
    C_Macro = lexer.C_Macro
    C_Enum = lexer.C_Enum
    C_Try = lexer.C_Try
    C_Catch = lexer.C_Catch
    C_Then = lexer.C_Then
    C_Throws = lexer.C_Throws
    C_Throw = lexer.C_Throw
    C_Endtry = lexer.C_Endtry
    Block = lexer.Block
    AsyncBlock = lexer.AsyncBlock
    ResultBlock = lexer.ResultBlock
    T_Number = lexer.T_Number
    T_Nil = lexer.T_Nil
    T_Bool = lexer.T_Bool
    T_Int = lexer.T_Int
    T_Uint = lexer.T_Uint
    T_Float = lexer.T_Float
    T_Bigi = lexer.T_Bigi
    T_Bigf = lexer.T_Bigf
    T_String = lexer.T_String
    T_Map = lexer.T_Map
    T_Array = lexer.T_Array
    T_Any = lexer.T_Any
    T_Pointer = lexer.T_Pointer
    END_STATEMENTS = lexer.END_STATEMENTS
)

// Exception control flow return values
const (
    EXCEPTION_HANDLED = iota // Exception was caught and handled
    EXCEPTION_THROWN         // Exception occurred and needs to bubble up
    EXCEPTION_RETURN         // Exception handling caused a return statement
)

// Error style modes for error_style() function
const (
    ERROR_STYLE_PANIC     = iota // Standard Go panic/recover (default)
    ERROR_STYLE_EXCEPTION        // Convert panics to exceptions
    ERROR_STYLE_MIXED            // Both panic and exception handling
)
