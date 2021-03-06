package main

import (
    str "strings"
    "strconv"
    "os"
)


const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const alphaplus = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_@$" // {}
const alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const numeric = "0123456789.f"
const identifier_set = alphanumeric + "_" // "{}"
const doubleterms = "<>=|&-+*.?"
const soloChars   = "+-/*.^!%;<>~=|,():[]&{}"
const expExpect="0123456789-+"

var tokNames = [...]string{"ERROR", "EOL", "EOF",
    "S_LITERAL", "N_LITERAL", "IDENTIFIER",
    "OPERATOR", "S_COMMENT",
    "PLUS", "MINUS", "DIVIDE", "MULTIPLY",
    "CARET", "PLING", "PERCENT", "SEMICOLON", "ASSIGN", "ASS_COMMAND", "LBRACE", "RBRACE","LCBRACE","RCBRACE",
    "PLUSEQ", "MINUSEQ", "MULEQ", "DIVEQ", "MODEQ", "LPAREN", "RPAREN",
    "SYM_EQ", "SYM_LT", "SYM_LE", "SYM_GT", "SYM_GE", "SYM_NE",
    "SYM_LAND", "SYM_LOR", "SYM_BAND", "SYM_BOR", "SYM_DOT", "SYM_PP", "SYM_MM", "SYM_POW", "SYM_RANGE",
    "SYM_LSHIFT", "SYM_RSHIFT","SYM_COLON", "COMMA", "TILDE", "ITILDE", "FTILDE", "SQR", "SQRT",
    "O_QUERY", "O_FILTER", "O_MAP","O_INFILE","O_REF","O_MUT","O_LC","O_UC","O_ST","O_LT","O_RT",
    "START_STATEMENTS", "VAR", "SETGLOB",
    "INIT", "IN", "PAUSE", "HELP", "NOP", "HIST", "DEBUG", "REQUIRE", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "SHOWDEF", "ENDDEF", "RETURN", "ASYNC",
    "LIB", "MODULE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "IF", "ELSE", "ENDIF", "WHEN",
    "IS", "CONTAINS", "HAS", "OR", "ENDWHEN", "WITH", "ENDWITH", "STRUCT", "ENDSTRUCT", "SHOWSTRUCT",
    "PANE", "DOC", "TEST", "ENDTEST", "ASSERT", "ON", "TO", "STEP", "AS", "DO","ENUM",
}

/// get the next available token, as a struct, from a given string and starting position.
func nextToken(input string, curLine *int16, start int, previousToken uint8) (carton Token, startNextTokenAt int, eol bool, eof bool) {

    var tokType uint8
    var word string
    var matchQuote bool
    var nonterm string
    var term string
    var firstChar byte
    var secondChar byte
    var twoChars bool
    var symword string
    var norepeat string
    var norepeatMap = make(map[byte]int)
    var badFloat,scientific,expectant bool

    beforeE := "."
    thisWordStart := -1

    // skip past whitespace
    lenInput:=len(input)
    var currentChar int
    for currentChar = start; currentChar<lenInput ; currentChar++ {
        if input[currentChar] == ' ' || input[currentChar]=='\r' || input[currentChar] == '\t' {
            continue
        }
        break
    }
    thisWordStart = currentChar

    // return \n as EOL - parser will figure the current line out
    if input[thisWordStart]=='\n' {
        carton.tokType=EOL
        eol=true
        startNextTokenAt=thisWordStart+1
        goto get_nt_exit_point
    }

    // abrupt endings...
    if currentChar>=lenInput {
        startNextTokenAt  = -1
        carton.tokType = EOF
        eof=true
        carton.tokText = ""
        goto get_nt_exit_point
    }

    // set word terminator depending on first char
    firstChar = input[thisWordStart]
    if thisWordStart < (lenInput-1) {
        secondChar = input[thisWordStart+1]
        twoChars = true
    }

    // some special cases
    if twoChars {

        c1 := str.IndexByte(doubleterms, firstChar)
        if c1!=-1 && firstChar==secondChar {
            word = string(firstChar)+string(secondChar)
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        }

        symword = string(firstChar)+string(secondChar)
        switch symword {
        case "?>": // list filter
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "<-": // file read
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "->": // maps to
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "!=":
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "<=":
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case ">=":
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "=|":
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "=@":
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "~i","~f":
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        case "-=","+=","*=","/=","%=":
            word=symword
            startNextTokenAt=thisWordStart+2
            goto get_nt_eval_point
        }
    }

    if firstChar == '#' {
        tokType = SingleComment
        nonterm = ""
        eol=true
        term = "\n"
    }

    // number
    if str.IndexByte(numeric, firstChar) != -1 && firstChar!='f' {
        tokType = NumericLiteral
        nonterm = numeric+"eE"
        term = "\n;"
        norepeat= "eE."
    }

    // solo symbols
    if str.IndexByte(soloChars, firstChar)!=-1 {
        word = string(firstChar)
        startNextTokenAt=thisWordStart+1
        goto get_nt_eval_point
    }

    // identifier or statement
    if str.IndexByte(alphaplus, firstChar) != -1 {
        nonterm = identifier_set
        term = "\n;"
    }

    // string literal
    if firstChar == '"' || firstChar == '`' || firstChar == '\'' {
        matchQuote = true
        tokType = StringLiteral
        term = string(firstChar)
        nonterm = ""
    }


    // start looking for word endings, (terms+nonterms)

    for currentChar = thisWordStart + 1; currentChar < lenInput; currentChar++ {

        // check numbers for illegal repeated chars
        if tokType==NumericLiteral {
            if expectant {
                if str.IndexByte(expExpect,input[currentChar])==-1 {
                    // wanted a digit / + / - here, but didn't find
                    word=input[thisWordStart:currentChar]
                    startNextTokenAt=currentChar
                    badFloat=true
                    break
                } else {
                    expectant=false
                    continue // skip past the char as it is legitimate.
                }
            }

            if str.IndexByte(beforeE,input[currentChar])>=0 && scientific {
                pf("Problem lexing character %c in '%s'\n",input[currentChar],str.TrimRight(input,"\n"))
                os.Exit(ERR_LEX)
            }

            if str.IndexByte(norepeat,input[currentChar])>=0 {
                var tu byte
                tu=input[currentChar]
                // special cases:
                switch input[currentChar] {
                case 'E':
                    scientific=true
                    expectant=true
                case 'e':
                    scientific=true
                    expectant=true
                    tu='E'
                }
                norepeatMap[input[currentChar]]++
                if norepeatMap[tu]>1 {
                    // end word at char before
                    word=input[thisWordStart:currentChar]
                    startNextTokenAt=currentChar
                    badFloat=true
                    break
                }
                // deal with . at end of number
                if currentChar<lenInput-1 && input[currentChar]=='.' && input[currentChar+1]=='.' {
                    word=input[thisWordStart:currentChar]
                    startNextTokenAt=currentChar
                    break
                }
            }

            // deal with 'f' at end of number
            if input[currentChar]=='f' {
                word=input[thisWordStart:currentChar+1]
                startNextTokenAt=currentChar+1
                break
            }

        }

        if matchQuote && input[currentChar]=='\n' {
            (*curLine)++
        }

        if nonterm != "" && str.IndexByte(nonterm, input[currentChar]) == -1 {
            // didn't find a non-terminator, so get word and finish, but don't
            // increase word end position as we need to continue the next
            // search from immediately after the word.
            word = input[thisWordStart:currentChar]
            startNextTokenAt=currentChar
            break
        }

        if term != "" && str.IndexByte(term, input[currentChar]) != -1 {
            // found a terminator character

            if tokType == SingleComment {
                carton.tokType = SingleComment
                carton.tokText = input[thisWordStart:currentChar]
                startNextTokenAt=currentChar
                goto get_nt_exit_point
            }

            if matchQuote {
                // get word and end, include terminal quote
                startNextTokenAt=currentChar+1
                carton.tokType= StringLiteral
                carton.tokText= input[thisWordStart:currentChar+1]
                // unescape escapes
                carton.tokText=str.Replace(carton.tokText, `\n`, "\n", -1)
                carton.tokText=str.Replace(carton.tokText, `\r`, "\r", -1)
                carton.tokText=str.Replace(carton.tokText, `\t`, "\t", -1)
                carton.tokText=str.Replace(carton.tokText, `\x`, "\\x", -1)
                carton.tokText=str.Replace(carton.tokText, `\\`, "\\", -1)
                carton.tokText=str.Replace(carton.tokText, `\"`, "\"", -1)
                goto get_nt_exit_point
            } else {
                // found a terminator, so get word and end.
                // we need to start next search on this terminator as
                // it wasn't part of the previous word.
                word = input[thisWordStart:currentChar]
                startNextTokenAt=currentChar
                break
            }
        }
    }

    // catch any eol strays
    if currentChar<lenInput {
        if !matchQuote && input[currentChar] == '\n' {
            startNextTokenAt=currentChar
            carton.tokText = input[thisWordStart:currentChar]
        }
    }

    // skip past empty word results
    if word == "" {
        word = input[thisWordStart:]
        eof = true
    }

    // if we have found a word match at this point, then bail with the result.
    // otherwise continue on to the switch to match keywords.

    if tokType != 0 {

        if tokType==NumericLiteral {
            if badFloat {
                tokType=StringLiteral
                carton.tokVal=word
            } else {
                tl:=str.ToLower(word)
                switch {
                case tl[len(tl)-1]=='f':
                    carton.tokVal,_=strconv.ParseFloat(tl[:len(tl)-1],64)
                    startNextTokenAt = currentChar+1
                    carton.tokType = tokType
                    carton.tokText = word
                    goto get_nt_exit_point
                case str.IndexByte(tl,'e')!=-1:
                    carton.tokVal,_=strconv.ParseFloat(tl,64)
                case str.IndexByte(tl,'.')!=-1:
                    carton.tokVal,_=strconv.ParseFloat(tl,64)
                default:
                    carton.tokVal,_=strconv.ParseInt(tl,10,0)
                    carton.tokVal=int(carton.tokVal.(int64))
                }
            }
        }

        startNextTokenAt = currentChar
        carton.tokType = tokType
        carton.tokText = word
        goto get_nt_exit_point

    }


get_nt_eval_point:

    // figure token type:

    switch word {
    case "+":
        tokType = O_Plus
    case "-":
        tokType = O_Minus
    case "/":
        tokType = O_Divide
    case "*":
        tokType = O_Multiply
    case "%":
        tokType = O_Percent
    case "^":
        tokType = SYM_Caret
    case "!":
        tokType = SYM_Not
    case ";":
        tokType = SYM_Semicolon
    case "[":
        tokType = LeftSBrace
    case "]":
        tokType = RightSBrace
    case "{":
        tokType = LeftCBrace
    case "}":
        tokType = RightCBrace
    case "+=":
        tokType = SYM_PLE
    case "-=":
        tokType = SYM_MIE
    case "*=":
        tokType = SYM_MUE
    case "/=":
        tokType = SYM_DIE
    case "%=":
        tokType = SYM_MOE
    case "(":
        tokType = LParen
    case ")":
        tokType = RParen
    case ",":
        tokType = O_Comma
    case "=":
        tokType = O_Assign
    case "~":
        tokType = SYM_Tilde
    case "~i":
        tokType = SYM_ITilde
    case "~f":
        tokType = SYM_FTilde
    case "??":
        tokType = O_Query
    case "?>":
        tokType = O_Filter
    case "->":
        tokType = O_Map
    case "<-":
        tokType = O_InFile
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
    case "..":
        tokType = SYM_RANGE
    case ".":
        tokType = SYM_DOT
    case "<<":
        tokType = SYM_LSHIFT
    case ">>":
        tokType = SYM_RSHIFT
    case ":":
        tokType = SYM_COLON
    case "=|":
        tokType = O_AssCommand
    }

    if tokType==0 {
        switch str.ToLower(word) {
        case "var":
            tokType = C_Var
        case "sqr":
            tokType = O_Sqr
        case "sqrt":
            tokType = O_Sqrt
        case "ref":
            tokType = O_Ref
        case "mut":
            tokType = O_Mut
        case "$lc":
            tokType = O_Slc
        case "$uc":
            tokType = O_Suc
        case "$st":
            tokType = O_Sst
        case "$lt":
            tokType = O_Slt
        case "$rt":
            tokType = O_Srt
        case "enum":
            tokType = C_Enum
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
        case "def","define":
            tokType = C_Define
        case "showdef":
            tokType = C_Showdef
        case "end","enddef":
            tokType = C_Enddef
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
        case "has":
            tokType = C_Has
        case "in":
            tokType = C_In
        case "or":
            tokType = C_Or
        case "and":
            tokType = SYM_LAND
        case "not":
            tokType = SYM_Not
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
        case "to":
            tokType = C_To
        case "step":
            tokType = C_Step
        case "as":
            tokType = C_As
        case "do":
            tokType = C_Do
        }
    }

    if tokType == 0 { // assume it was an identifier
        tokType = Identifier
        startNextTokenAt=currentChar
        if strcmp(word,"true")  { carton.subtype=subtypeConst ; carton.tokVal=true }
        if strcmp(word,"false") { carton.subtype=subtypeConst ; carton.tokVal=false }
        if strcmp(word,"nil")   { carton.subtype=subtypeConst ; carton.tokVal=nil }
    }

    carton.tokType = tokType
    carton.tokText = word


get_nt_exit_point:
    // you have to set carton.tokType + startNextTokenAt by hand if you jump
    // directly to this exit point.

    if startNextTokenAt>=lenInput { eof=true }

    return carton, startNextTokenAt, eol, eof

}

