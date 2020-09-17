package main

import (
    str "strings"
   "strconv"
)


const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const alphaplus = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_@{}"
const alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const numeric = "0123456789."
const identifier_set = alphanumeric + "_{}"
const doubleterms = "<>=|&-+*"

// const symbols     = "<>=!|&.+-*"
const soloChars   = "+-/*.^!%;<>~=|,():[]&"

var tokNames = [...]string{"ERROR", "ESCAPE",
    "S_LITERAL", "N_LITERAL", "IDENTIFIER",
    "EXPRESSION", "OPTIONAL_EXPRESSION", "OPERATOR",
    "S_COMMENT", "D_COMMENT", "PLUS", "MINUS", "DIVIDE", "MULTIPLY",
    "CARET", "PLING", "PERCENT", "SEMICOLON", "LBRACE", "RBRACE", "LPAREN", "RPAREN",
    "SYM_EQ", "SYM_LT", "SYM_LE", "SYM_GT", "SYM_GE", "SYM_NE",
    "SYM_LAND", "SYM_LOR", "SYM_BAND", "SYM_BOR", "SYM_DOT", "SYM_PP", "SYM_MM", "SYM_POW",
    "SYM_LSHIFT", "SYM_RSHIFT","SYM_COLON", "COMMA", "TILDE",
    "START_STATEMENTS", "VAR", "ASSIGN", "SETGLOB", "ZERO", "INC", "DEC", "ASS_COMMAND",
    "R_COMMAND", "INIT", "PAUSE", "HELP", "NOP", "HIST", "DEBUG", "REQUIRE", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "ENDDEF", "SHOWDEF", "RETURN", "ASYNC",
    "LIB", "MODULE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "IF", "ELSE", "ENDIF", "WHEN",
    "IS", "CONTAINS", "IN", "OR", "ENDWHEN", "WITH", "ENDWITH", "STRUCT", "ENDSTRUCT", "SHOWSTRUCT",
    "PANE", "DOC", "TEST", "ENDTEST", "ASSERT", "ON", "EOL", "EOF",
}


/*
type TokenCache struct {
	s   string
	t   Token
    sp  int
    eol bool
    eof bool
}
*/


/// get the next available token, as a struct, from a given string and starting position.
func nextToken(input string, curLine *int, start int, previousToken uint8, newStatement bool) (carton Token, eol bool, eof bool) {

    var tokType uint8
    var word string
    var endPos int
    var matchQuote bool
    var slashComment bool
    var backtrack int // push back so that eol can be processed.
    var nonterm string
    var term string
    // var doublesymbol bool
    var firstChar byte
    var secondChar byte
    var two bool
    var symword string

    // skip past whitespace
    skip := -1


    li:=len(input)
    var i int
    for i = start; i<li ; i++ {
        if input[i] == ' ' || input[i]=='\r' || input[i] == '\t' {
            continue
        }
        break
    }
    skip = i

    // @note: will this ever be true?
    if skip == -1 {
        carton.tokPos = -1
        carton.Line = *curLine
        carton.tokType = EOF
        carton.tokText = ""
        goto get_nt_exit_point
    }

    // bad endings...
    if skip>=li {
        carton.tokPos  = -1
        carton.Line    = *curLine
        carton.tokType = EOL
        carton.tokText = ""
        goto get_nt_exit_point
    }

    // set word terminator depending on first char

    firstChar = input[skip]
    if skip < (li-1) {
        secondChar = input[skip+1]
        two = true
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

        // some special cases

        c1 := str.IndexByte(doubleterms, firstChar)
        if c1!=-1 && firstChar==secondChar {
                    word = string(firstChar)+string(secondChar)
                    endPos=skip+1
                    goto get_nt_eval_point
        }

        symword = string(firstChar)+string(secondChar)
        switch symword {
        case "!=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "<=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case ">=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "=|":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "=@":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "-=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "+=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "*=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "/=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        case "%=":
            word=symword
            endPos=skip+1
            goto get_nt_eval_point
        }
    }

    switch firstChar {
    case '\n':
        eol = true
        carton.tokPos = skip
        carton.Line = *curLine
        (*curLine)++
        carton.tokType = EOL
        goto get_nt_exit_point
    case '#':
            tokType = SingleComment
            nonterm = ""
            term = "\n"
            backtrack = 1
    }

    // number
    if str.IndexByte(numeric, firstChar) != -1 {
        tokType = NumericLiteral
        nonterm = numeric+"e"
        term = ""
    }

        /*
        // symbols
        if two {
            // treat double symbol as a keyword
            c1 := str.IndexByte(symbols, firstChar)
            c2 := str.IndexByte(doubleterms, secondChar)
            if c1 != -1 && c2 != -1 {
                    word = string(firstChar)+string(secondChar)
                    endPos=skip+1
                    goto get_nt_eval_point
            }
        }
        */

        // solo symbols
        if !slashComment {
            c := str.IndexByte(soloChars, firstChar)
            if c != -1 {
                    word = string(firstChar)
                    endPos=skip
                    goto get_nt_eval_point
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
        tokType = StringLiteral
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

        if matchQuote && input[i]=='\n' {
            (*curLine)++
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
                goto get_nt_exit_point
            }

            // flag another EOL in count
            if input[i]=='\n' { (*curLine)++ }

            if matchQuote {
                // get word and end, include terminal quote
                carton.tokPos = endPos
                carton.Line   = *curLine
                carton.tokType= StringLiteral
                carton.tokText= input[skip:i+1]
                // unescape escapes
                carton.tokText=str.Replace(carton.tokText, `\n`, "\n", -1)
                carton.tokText=str.Replace(carton.tokText, `\r`, "\r", -1)
                carton.tokText=str.Replace(carton.tokText, `\t`, "\t", -1)
                carton.tokText=str.Replace(carton.tokText, `\\`, "\\", -1)
                carton.tokText=str.Replace(carton.tokText, `\"`, "\"", -1)
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
        goto get_nt_exit_point
    }

    if tokType==SingleComment {
        carton.tokPos = endPos - backtrack
        carton.Line = *curLine
        carton.tokType = SingleComment
        carton.tokText = input[skip:i]
        eol=true
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
        if tokType==NumericLiteral {
            if str.IndexByte(str.ToLower(word), 'e') != -1 || str.IndexByte(str.ToLower(word), '.') != -1 {
                carton.tokVal,_=strconv.ParseFloat(word,64)
            } else {
                carton.tokVal,_=strconv.ParseInt(word,10,0)
                carton.tokVal=int(carton.tokVal.(int64))
            }
        }
        carton.tokPos = endPos - backtrack
        carton.Line = *curLine
        carton.tokType = tokType
        carton.tokText = word
        goto get_nt_exit_point
    }


get_nt_eval_point:

    // figure token type:
    //  needs tidying.. some aren't used now.

    // deal with symbols that don't require a case conversion first, saves some cycles

    switch word {
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
    case "(":
        tokType = LParen
    case ")":
        tokType = RParen
    case ",":
        tokType = C_Comma
    case "=":
        tokType = C_Assign
    case "~":
        tokType = C_Tilde
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
    case "&&":
        tokType = SYM_LAND
    case "||":
        tokType = SYM_LOR
    case "&":
        tokType = SYM_BAND
    case "|":
        tokType = SYM_BOR
    case "++":
        tokType = SYM_PP
    case "--":
        tokType = SYM_MM
    case "**":
        tokType = SYM_POW
    case ".":
        tokType = SYM_DOT
    case "<<":
        tokType = SYM_LSHIFT
    case ">>":
        tokType = SYM_RSHIFT
    case ":":
        tokType = SYM_COLON
    case "=|":
        tokType = C_AssCommand
    case "|@":
        tokType = C_RemoteCommand
    }

    if tokType==0 {
        switch str.ToLower(word) {
        case "zero":
            tokType = C_Zero
        case "var":
            tokType = C_Var
        case "inc":
            tokType = C_Inc
        case "dec":
            tokType = C_Dec
        case "init":
            tokType = C_Init
        case "setglob":
            tokType = C_SetGlob
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
        case "async":
            tokType = C_Async
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
        case "with":
            tokType = C_With
        case "endwith":
            tokType = C_Endwith
        case "struct":
            tokType = C_Struct
        case "endstruct":
            tokType = C_Endstruct
        case "showstruct":
            tokType = C_Showstruct
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
    }

    if tokType == 0 { // assume it was an identifier
        tokType = Identifier
    }

    // box up the token
    carton.tokPos = endPos - backtrack
    carton.Line = *curLine
    carton.tokType = tokType
    carton.tokText = word

get_nt_exit_point:

    return carton, eol, eof

}

