package main

import (
    str "strings"
)

const symbols = "<>=!|&.+-"
const doubleterms = "<>=!|&"
const soloChars = "+-/*^!%;<>~=|,[]&"

var soloBinds = [...]int{C_Plus, C_Minus, C_Divide, C_Multiply, C_Caret, C_Pling, C_Percent, C_Semicolon, SYM_LT, SYM_GT, C_Tilde, C_Assign, C_LocalCommand, C_Comma, LeftSBrace, RightSBrace,SYM_AMP}

// const identifier_set = alphanumeric + "_~.{}[\"]"
const identifier_set = alphanumeric + "_~.{}[]"

var tokNames = [...]string{"ERROR", "ESCAPE",
    "S_LITERAL", "N_LITERAL", "IDENTIFIER",
    "EXPRESSION", "OPTIONAL_EXPRESSION", "OPERATOR",
    "S_COMMENT", "D_COMMENT", "PLUS", "MINUS", "DIVIDE", "MULTIPLY",
    "CARET", "PLING", "PERCENT", "SEMICOLON", "LBRACE", "RBRACE",
    "SYM_EQ", "SYM_LT", "SYM_LE", "SYM_GT", "SYM_GE", "SYM_NE", "SYM_AMP",
    "COMMA", "TILDE", "ASSIGN", "SETGLOB", "ZERO", "INC", "DEC", "ASS_COMMAND", "L_COMMAND",
    "R_COMMAND", "INIT", "INSTALL", "PUSH", "TRIGGER", "DOWNLOAD", "PAUSE",
    "HELP", "NOP", "HIST", "DEBUG", "REQUIRE", "DEPENDS", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "INDENT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "ENDDEF", "SHOWDEF", "RETURN",
    "LIB", "MODULE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "IF", "ELSE", "ENDIF", "WHEN",
    "IS", "CONTAINS", "IN", "OR", "ENDWHEN", "PANE", "DOC", "TEST", "ENDTEST", "ASSERT", "ON", "EOL", "EOF",
}


type TokenCache struct {
	s   string
	t   Token
    sp  int
    eol bool
    eof bool
}
var lasttoken TokenCache

//
// n.b. this caching will rarely be useful, as nextToken returns
//  after each token, so start pos will creep through the line
//  before you get a repeat input usually.
//  
// will leave it in place for now, but may return it to sane returns
//  instead of gotos one day.
// 
// caching would be of more use in caller to nextToken()
//


/// get the next available token, as a struct, from a given string and starting position.
func nextToken(input string, curLine *int, start int, previousToken int) (carton Token, eol bool, eof bool) {

    var tokType int
    var word string
    var endPos int
    var matchQuote bool
    var slashComment bool
    var backtrack int // push back so that eol can be processed.
    var nonterm string
    var term string
    var doublesymbol bool
    var secondChar byte
    var two bool
    var firstChar byte
    var lt TokenCache

    // return cached result if available
    if !lockSafety {
        lt=lasttoken
        if input==lt.s && lt.sp==start {
            return lt.t, lt.eol, lt.eof
        }
    }

    lt.s=input
    lt.sp=start

    // skip past whitespace
    skip := -1

    // simple handler for parens nesting
    var braceNestLevel  int     // round braces
    var sbraceNestLevel int     // square braces

    li:=len(input)
    var i int
    for i = start; i<li ; i++ {
        if input[i] == ' ' || input[i]=='\r' || input[i] == '\t' {
            continue
        }
        break
    }
    skip = i

    if skip == -1 {
        carton.tokPos = -1
        carton.Line = *curLine
        carton.tokType = EOF
        carton.tokText = ""
        lt.t=carton; lt.eol=true; lt.eof=true
        goto get_nt_exit_point
    }

    // bad endings...
    if skip>=li {
        carton.tokPos  = -1
        carton.Line    = *curLine
        carton.tokType = EOL
        carton.tokText = ""
        lt.t=carton; lt.eol=true; lt.eof=false
        goto get_nt_exit_point
    }

    // set word terminator depending on first char

    firstChar = input[skip]
    if skip < (li-1) {
        secondChar = input[skip+1]
        two = true
    }

    // newline in input
    if firstChar == '\n' {
        eol = true
        carton.tokPos = skip
        carton.Line = *curLine
        (*curLine)++
        carton.tokType = EOL
        lt.t=carton; lt.eol=eol; lt.eof=eof
        goto get_nt_exit_point
    }

    // comments
    if two {
        if (firstChar == '/') && (secondChar == '/') {
            tokType = SingleComment
            nonterm = ""
            term = "\n"
            backtrack = 1
            slashComment=true
        }
    }

    if firstChar == '#' {
            tokType = SingleComment
            nonterm = ""
            term = "\n"
            backtrack = 1
    }

    // square braced expression
    if firstChar == '[' {
        tokType = Expression
        sbraceNestLevel++
        nonterm = ""
        term = "]"
    }

    // braced expression
    if firstChar == '(' {
        tokType = Expression
        braceNestLevel++
        nonterm = ""
        term = ")"
    }

    // number
    if str.IndexByte(numeric, firstChar) != -1 {
        tokType = NumericLiteral
        nonterm = numeric
        term = ""
    }

    // symbols
    if two {
        // treat double symbol as a keyword
        c1 := str.IndexByte(symbols, firstChar)
        c2 := str.IndexByte(doubleterms, secondChar)
        if c1 != -1 && c2 != -1 {
            nonterm = doubleterms
            doublesymbol = true
        }
    }

    // solo symbols
    if !slashComment && !doublesymbol {
        c := str.IndexByte(soloChars, firstChar)
        if c != -1 {
            tokType = soloBinds[c]
            nonterm = string(firstChar)
        }
    }

    // identifier or statement
    if str.IndexByte(alphaplus, firstChar) != -1 {
        nonterm = identifier_set
        term = ""
    }

    // string literal
    if firstChar == '"' || firstChar == '`' || firstChar == '\'' {
        matchQuote = true
        tokType = Expression
        term = string(firstChar)
        nonterm = ""
    }

    // expression?
    if tokType != SingleComment && term == "" && nonterm == "" {
        tokType = Expression
        term = ";\n"
        backtrack = 1
    }

    for i = skip + 1; i < li; i++ {

        endPos = i

        if !matchQuote && input[i]=='\n' && ( sbraceNestLevel>0 || braceNestLevel>0 ) {
            (*curLine)++
        }

        if term == "]" {

            if input[i] == '[' {
                sbraceNestLevel++
            }
            if input[i] == ']' {
                sbraceNestLevel--
            }

            if sbraceNestLevel > 0 {
                continue
            }

            if input[i] == ']' {
                carton.tokPos = i
                carton.Line = *curLine
                carton.tokType = tokType
                carton.tokText = input[skip : i+1]
                lt.t=carton; lt.eol=eol; lt.eof=eof
                goto get_nt_exit_point
            }

        }

        if term == ")" {

            if input[i] == '(' {
                braceNestLevel++
            }
            if input[i] == ')' {
                braceNestLevel--
            }

            if braceNestLevel > 0 {
                continue
            }

            if input[i] == ')' {
                carton.tokPos = i
                carton.Line = *curLine
                carton.tokType = tokType
                carton.tokText = input[skip : i+1]
                lt.t=carton; lt.eol=eol; lt.eof=eof
                goto get_nt_exit_point
            }

        }

        if nonterm != "" && str.IndexByte(nonterm, input[i]) == -1 {
                // didn't find a non-terminator, so get word and finish
                // but don't increase skip as we need to continue the next
                // search from immediately after the word.
                word = input[skip:i]
                endPos--
                break
        }

        if term != "" && str.IndexByte(term, input[i]) != -1 {
            // found a terminator character

            if tokType == SingleComment {
                carton.tokPos = endPos - backtrack
                carton.Line = *curLine
                carton.tokType = SingleComment
                carton.tokText = ""
                eol=true
                lt.t=carton; lt.eol=eol; lt.eof=eof
                goto get_nt_exit_point
            }

            // flag another EOL in count
            if input[i]=='\n' { (*curLine)++ }

            if matchQuote {
                // get word and end, include terminal quote
                // get word and end, don't include quotes
                carton.tokPos = endPos
                carton.Line   = *curLine
                carton.tokType= Expression
                carton.tokText= input[skip:i+1]
                lt.t=carton; lt.eol=false; lt.eof=false
                goto get_nt_exit_point
                // break
            } else {
                // found a terminator, so get word and end.
                // we need to start next search on this terminator as
                // it wasn't part of the previous word.
                word = input[skip:i]
                break
            }
        }

    }

    // catch any eol strays - these can come from non-terms above.
    if !matchQuote && input[endPos] == '\n' {
        eol = true
        carton.tokPos = endPos
        carton.Line = *curLine
        carton.tokType = EOL
        carton.tokText = input[skip:endPos]
        lt.t=carton; lt.eol=eol; lt.eof=eof
        goto get_nt_exit_point
    }

    if tokType==SingleComment {
        carton.tokPos = endPos - backtrack
        carton.Line = *curLine
        carton.tokType = SingleComment
        carton.tokText = input[skip:i]
        eol=true
        lt.t=carton; lt.eol=eol; lt.eof=eof
        goto get_nt_exit_point
    }


    // skip past empty word results
    if word == "" {
            word = input[skip:]
        eof = true
    }

    // if we have found a word match at this point, then bail with the result.
    // otherwise continue on to the switch to match keywords.

    if tokType != 0 {
        carton.tokPos = endPos - backtrack
        carton.Line = *curLine
        carton.tokType = tokType
        carton.tokText = word
        lt.t=carton; lt.eol=eol; lt.eof=eof
        goto get_nt_exit_point
    }

    // figure token type:
    switch str.ToLower(word) {
    // EscapeSequence
    case "zero":
        tokType = C_Zero
    case "inc":
        tokType = C_Inc
    case "dec":
        tokType = C_Dec
    case "+":
        tokType = C_Plus
    case "-":
        tokType = C_Minus
    case "/":
        tokType = C_Divide
    case "*":
        tokType = C_Multiply
    case "%":
        tokType = C_Percent
    case "^":
        tokType = C_Caret
    case "!":
        tokType = C_Pling
    case ";":
        tokType = C_Semicolon
    case "[":
        tokType = LeftSBrace
    case "]":
        tokType = RightSBrace
    case "=":
        tokType = C_Assign
    case "<":
        tokType = SYM_LT
    case ">":
        tokType = SYM_GT
    case "==":
        tokType = SYM_EQ
    case "<=":
        tokType = SYM_LE
    case ">=":
        tokType = SYM_GE
    case "!=":
        tokType = SYM_NE
    case "=|":
        tokType = C_AssCommand
    case "|":
        tokType = C_LocalCommand
    case "|@":
        tokType = C_RemoteCommand
    case "init":
        tokType = C_Init
    case "setglob":
        tokType = C_SetGlob
    case "install":
        tokType = C_Install
    case "push":
        tokType = C_Push
    case "trigger":
        tokType = C_Trigger
    case "download":
        tokType = C_Download
    case "pause":
        tokType = C_Pause
    case "help":
        tokType = C_Help
    case "nop":
        tokType = C_Nop
    case "hist":
        tokType = C_Hist
    case "debug":
        tokType = C_Debug
    case "require":
        tokType = C_Require
    case "depends":
        tokType = C_Depends
    case "exit":
        tokType = C_Exit
    case "version":
        tokType = C_Version
    case "quiet":
        tokType = C_Quiet
    case "loud":
        tokType = C_Loud
    case "unset":
        tokType = C_Unset
    case "input":
        tokType = C_Input
    case "prompt":
        tokType = C_Prompt
    case "indent":
        tokType = C_Indent
    case "log":
        tokType = C_Log
    case "print":
        tokType = C_Print
    case "println":
        tokType = C_Println
    case "logging":
        tokType = C_Logging
    case "cls":
        tokType = C_Cls
    case "at":
        tokType = C_At
    case "define":
        tokType = C_Define
    case "enddef":
        tokType = C_Enddef
    case "showdef":
        tokType = C_Showdef
    case "return":
        tokType = C_Return
    case "lib":
        tokType = C_Lib
    case "module":
        tokType = C_Module
    case "uses":
        tokType = C_Uses
    case "while":
        tokType = C_While
    case "endwhile":
        tokType = C_Endwhile
    case "for":
        tokType = C_For
    case "foreach":
        tokType = C_Foreach
    case "endfor":
        tokType = C_Endfor
    case "continue":
        tokType = C_Continue
    case "break":
        tokType = C_Break
    case "if":
        tokType = C_If
    case "else":
        tokType = C_Else
    case "endif":
        tokType = C_Endif
    case "when":
        tokType = C_When
    case "is":
        tokType = C_Is
    case "contains":
        tokType = C_Contains
    case "in":
        tokType = C_In
    case "or":
        tokType = C_Or
    case "endwhen":
        tokType = C_Endwhen
    case "pane":
        tokType = C_Pane
    case "doc":
        tokType = C_Doc
    case "test":
        tokType = C_Test
    case "endtest":
        tokType = C_Endtest
    case "assert":
        tokType = C_Assert
    case "on":
        tokType = C_On
    }

    if tokType == 0 { // assume it was an identifier
        tokType = Identifier
    }

    // box up the token
    carton.tokPos = endPos - backtrack
    carton.Line = *curLine
    carton.tokType = tokType
    carton.tokText = word
    lt.t=carton; lt.eol=eol; lt.eof=eof

get_nt_exit_point:

    // only update cache if not at eol or eof
    if !lockSafety && eol==false && eof==false {
        lasttoken=lt
    }

    return carton, eol, eof

}
