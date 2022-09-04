package main

import (
    str "strings"
    "strconv"
    "math"
    "os"
)


const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const alphaplus = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_@$"
const alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const numeric = "0123456789.fn"
const numSeps = "_"
const identifier_set = alphanumeric + "_"
const doubleterms = "<>=|&-+*."
const expExpect="0123456789-+"

var tokNames = [...]string{"ERROR", "EOL", "EOF",
    "S_LITERAL", "N_LITERAL", "IDENTIFIER",
    "OPERATOR", "S_COMMENT",
    "PLUS", "MINUS", "DIVIDE", "MULTIPLY",
    "CARET", "PLING", "PERCENT", "SEMICOLON", "ASSIGN", "ASS_COMMAND", "ASS_OUT_COMMAND", "LBRACE", "RBRACE","LCBRACE","RCBRACE",
    "PLUSEQ", "MINUSEQ", "MULEQ", "DIVEQ", "MODEQ", "LPAREN", "RPAREN",
    "SYM_EQ", "SYM_LT", "SYM_LE", "SYM_GT", "SYM_GE", "SYM_NE",
    "SYM_LAND", "SYM_LOR", "SYM_BAND", "SYM_BOR", "SYM_BSLASH", "SYM_DOT", "SYM_PP", "SYM_MM", "SYM_POW", "SYM_RANGE",
    "SYM_LSHIFT", "SYM_RSHIFT","SYM_COLON", "COMMA", "TILDE", "ITILDE", "FTILDE", "SQR", "SQRT",
    "O_QUERY", "O_FILTER", "O_MAP","O_INFILE","O_OUTFILE","O_REF","O_MUT","O_LC","O_UC","O_ST","O_LT","O_RT",
    "O_PB","O_PA","O_PN","O_PE","O_PP",
    "START_STATEMENTS", "VAR", "SETGLOB",
    "INIT", "IN", "PAUSE", "HELP", "NOP", "HIST", "DEBUG", "REQUIRE", "EXIT", "VERSION",
    "QUIET", "LOUD", "UNSET", "INPUT", "PROMPT", "LOG", "PRINT", "PRINTLN",
    "LOGGING", "CLS", "AT", "DEFINE", "SHOWDEF", "ENDDEF", "RETURN", "ASYNC",
    "LIB", "MODULE", "USES", "WHILE", "ENDWHILE", "FOR", "FOREACH",
    "ENDFOR", "CONTINUE", "BREAK", "IF", "ELSE", "ENDIF", "WHEN",
    "IS", "CONTAINS", "HAS", "OR", "ENDWHEN", "WITH", "ENDWITH", "STRUCT", "ENDSTRUCT", "SHOWSTRUCT",
    "PANE", "DOC", "TEST", "ENDTEST", "ASSERT", "ON", "TO", "STEP", "AS", "DO","ENUM","BLOCK","ABLOCK","RBLOCK",
    "T_NUMBER", "T_NIL", "T_BOOL", "T_INT", "T_UINT", "T_FLOAT", "T_BIGI",
    "T_BIGF", "T_STRING", "T_MAP", "T_ARRAY", "T_ANY",
}

type lcstruct struct {
    carton Token;tokPos int;eol bool;eof bool;borpos int
}


/// get the next available token, as a struct, from a given string and starting position.
func nextToken(input string, fs uint32, curLine *int16, start int) (rv *lcstruct) {

    lenInput:=len(input)

    var carton Token
    var startNextTokenAt int
    var eol,eof bool
    var tokType uint8
    var word string
    var matchBlock bool
    var matchQuote bool
    // var matchComment,foundComment bool
    var nonterm string
    var term string
    var firstChar byte
    var secondChar byte
    var twoChars bool
    var norepeat string
    var norepeatMap = make(map[byte]int)
    var badFloat,scientific,expectant,hasPoint bool
    var maybeBaseChange, thisHex, thisOct, thisBin bool
    var blockBraceLevel int

    beforeE := "."
    thisWordStart := -1
    borpos := -1
    maybeBaseChange=true

    // skip past whitespace
    var currentChar int
    currentChar=start


// rescan: // used by /*...*/ comments


    for ; currentChar<lenInput ; currentChar+=1 {
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

    firstChar = input[thisWordStart]

    // string literal
    if firstChar == '"' || firstChar == '`' || firstChar == '\'' {
        matchQuote = true
        tokType = StringLiteral
        term = string(firstChar)
        nonterm = ""
    }

    if !matchQuote {

        // block
        if firstChar == '{' {
            matchBlock=true
            tokType=ResultBlock
            term="}"
            nonterm=""
            blockBraceLevel=1
        }

        // set word terminator depending on first char
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

            switch string(firstChar)+string(secondChar) {
            case "!=":
                word="!="
                startNextTokenAt=thisWordStart+2
                goto get_nt_eval_point
            case "<=":
                word="<="
                startNextTokenAt=thisWordStart+2
                goto get_nt_eval_point
            case ">=":
                word=">="
                startNextTokenAt=thisWordStart+2
                goto get_nt_eval_point
            case "-=","+=","*=","/=","%=","=<","=@","=|","->","~i","~f","?>":
                word=string(firstChar)+string(secondChar)
                startNextTokenAt=thisWordStart+2
                goto get_nt_eval_point
            case "${": //  block
                matchBlock=true
                tokType=Block
                term="}"
                nonterm=""
                currentChar+=1
                thisWordStart+=1
                blockBraceLevel=1
            case "&{": // async block
                matchBlock=true
                tokType=AsyncBlock
                term="}"
                nonterm=""
                currentChar+=1
                thisWordStart+=1
                blockBraceLevel=1
            }
        }


        if !matchBlock {

            if firstChar == '#' {
                tokType = SingleComment
                nonterm = ""
                eol=true
                term = "\n"
            }


            if firstChar == '.' {
                hasPoint=true
            }

            // number
            if firstChar!='f' && firstChar!='n' && str.IndexByte(numeric, firstChar) != -1 {
                tokType = NumericLiteral
                nonterm = numeric+"xeE"+numSeps
                term = "\n;"
                norepeat= "oxOXeE."
            }

            // solo symbols
            switch firstChar {
            case '+','-','/','*','.','^','!','%','?',';','<','>','~','=','|',',','(',')',':','[',']','&':
                word = string(firstChar)
                startNextTokenAt=thisWordStart+1
                goto get_nt_eval_point
            }

            // identifier or statement
            if str.IndexByte(alphaplus, firstChar) != -1 {
                nonterm = identifier_set
                term = "\n;"
            }

            if firstChar == '\\' {
                word = string(firstChar)
                startNextTokenAt=thisWordStart+1
                goto get_nt_eval_point
            }

        }

    } // eo-not-matchQuote


    // start looking for word endings, (terms+nonterms)

    for currentChar = thisWordStart + 1; currentChar < lenInput; currentChar+=1 {

        // check numbers for illegal repeated chars
        if tokType==NumericLiteral {

            if input[currentChar]=='.' {
                hasPoint=true
            }

            if str.IndexByte(numSeps,input[currentChar])!=-1 {
                continue
            }

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

            if scientific && str.IndexByte(beforeE,input[currentChar])>=0 {
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
                norepeatMap[input[currentChar]]+=1
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
                    hasPoint=true
                    startNextTokenAt=currentChar
                    break
                }
            }

            // deal with 'n' at end of number
            if input[currentChar]=='n' {
                word=input[thisWordStart:currentChar+1]
                startNextTokenAt=currentChar+1
                break
            }
            // deal with 'f' at end of number
            if !thisHex && input[currentChar]=='f' {
                word=input[thisWordStart:currentChar+1]
                startNextTokenAt=currentChar+1
                break
            }

            // deal with '0x' at start
            if maybeBaseChange && input[thisWordStart]=='0' && currentChar==thisWordStart+1 {
                switch input[currentChar] {
                case 'x','X':
                    thisHex=true
                    nonterm="0123456789abcdefABCDEFxX"
                case 'b','B':
                    thisBin=true
                    nonterm="01bB"
                case 'o','O':
                    thisOct=true
                    nonterm="01234567oO"
                }
            }
            if currentChar==thisWordStart+1 {
                maybeBaseChange=false
            }

        } // eo-numeric-literal

        if matchBlock && input[currentChar]=='{' {
            blockBraceLevel+=1
        }

        if matchBlock && input[currentChar]=='}' {
            blockBraceLevel-=1
            if blockBraceLevel>0 { continue }
        }

        if (matchBlock||matchQuote) && input[currentChar]=='\n' {
            (*curLine)+=1
        }

        if (matchBlock||matchQuote) && input[currentChar]=='\\' {
            // skip past
            continue
        }


        if nonterm != "" && str.IndexByte(nonterm, input[currentChar]) == -1 {
            // didn't find a non-terminator, so get word and finish, but don't
            // increase word end position as we need to continue the next
            // search from immediately after the word.
            word = input[thisWordStart:currentChar]
            startNextTokenAt=currentChar
            break
        }

        if len(term)!=0 && str.IndexByte(term, input[currentChar]) != -1 {
            // found a terminator character

            if matchBlock {
                carton.tokType = tokType
                carton.tokText  = input[thisWordStart+1:currentChar]
                startNextTokenAt= currentChar+1
                goto get_nt_exit_point
            }

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
                carton.tokText=stripBacktickQuotes(stripDoubleQuotes(carton.tokText))

                carton.tokText=str.Replace(carton.tokText, `\\`, "\\", -1)
                carton.tokText=str.Replace(carton.tokText, `\r`, "\r", -1)
                carton.tokText=str.Replace(carton.tokText, `\t`, "\t", -1)
                carton.tokText=str.Replace(carton.tokText, `\x`, "\\x", -1)
                carton.tokText=str.Replace(carton.tokText, `\u`, "\\u", -1)
                carton.tokText=str.Replace(carton.tokText, `\n`, "\n", -1)
                carton.tokText=str.Replace(carton.tokText, `\"`, "\"", -1)

                goto get_nt_exit_point
            } else {
                // found a terminator, so get word and end.
                // we need to start next search on this terminator as
                // it wasn't part of the previous word.
                if input[currentChar-1]!='\\' {
                    word = input[thisWordStart:currentChar]
                    startNextTokenAt=currentChar
                    break
                }
            }
        }
    }

    // catch any eol strays
    if currentChar<lenInput {
        if !(matchBlock||matchQuote) && input[currentChar] == '\n' {
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

            if thisHex {
                hs:=str.Replace(word,"0x","",-1)
                hs=str.Replace(hs,"0X","",-1)
                carton.tokVal,_=strconv.ParseInt(hs,16,64)
                startNextTokenAt=currentChar
                carton.tokType=NumericLiteral
                carton.tokText=word
                goto get_nt_exit_point
            }

            if thisOct {
                hs:=str.Replace(word,"0o","",-1)
                hs=str.Replace(hs,"0O","",-1)
                carton.tokVal,_=strconv.ParseInt(hs,8,64)
                startNextTokenAt=currentChar
                carton.tokType=NumericLiteral
                carton.tokText=word
                goto get_nt_exit_point
            }

            if thisBin {
                hs:=str.Replace(word,"0b","",-1)
                hs=str.Replace(hs,"0B","",-1)
                carton.tokVal,_=strconv.ParseInt(hs,2,64)
                startNextTokenAt=currentChar
                carton.tokType=NumericLiteral
                carton.tokText=word
                goto get_nt_exit_point
            }

            // remove any numSeps from literal
            for _,ns:=range numSeps { word=str.Replace(word,string(ns),"",-1) }

            // floats and big nums
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

                case tl[len(tl)-1]=='n':
                    if hasPoint {
                        carton.tokVal=GetAsBigFloat(tl[:len(tl)-1])
                    } else {
                        carton.tokVal=GetAsBigInt(tl[:len(tl)-1])
                    }
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
/*
    case "{":
        tokType = LeftCBrace
    case "}":
        tokType = RightCBrace
*/
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
    case "?":
        tokType = O_Query
    case "?>":
        tokType = O_Filter
    case "->":
        tokType = O_Map
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
    case `\`:
        tokType = SYM_BSLASH
    case "|":
        tokType = SYM_BOR
        borpos  = thisWordStart
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
        borpos  = thisWordStart
    case "=<":
        tokType = O_AssOutCommand
        borpos  = thisWordStart
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
        case "$in":
            tokType = O_InFile
        case "$out":
            tokType = O_OutFile
        case "$pb":
            tokType = O_Pb
        case "$pa":
            tokType = O_Pa
        case "$pn":
            tokType = O_Pn
        case "$pe":
            tokType = O_Pe
        case "$pp":
            tokType = O_Pp
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
        case "number":
            tokType = T_Number
        case "nil":
            tokType = T_Nil
        case "bool":
            tokType = T_Bool
        case "int":
            tokType = T_Int
        case "uint":
            tokType = T_Uint
        case "float":
            tokType = T_Float
        case "bigi":
            tokType = T_Bigi
        case "bigf":
            tokType = T_Bigf
        case "string":
            tokType = T_String
        case "map":
            tokType = T_Map
        case "array":
            tokType = T_Array
        case "any":
            tokType = T_Any
        }
    }

    if tokType == 0 { // assume it was an identifier
        tokType = Identifier
        startNextTokenAt=currentChar

        // add token's bind_int value
        if len(word)>0 {
            bin:=bind_int(fs,word)
            carton.bindpos=bin
            carton.bound=true
            // pf("[#3]lex:bound %s in fs %d with bin %d[#-]\n",word,fs,bin)
        }

        if strcmp(word,"true")  { carton.subtype=subtypeConst ; carton.tokVal=true }
        if strcmp(word,"false") { carton.subtype=subtypeConst ; carton.tokVal=false }
        if strcmp(word,"nil")   { carton.subtype=subtypeConst ; carton.tokVal=nil }
        if strcmp(word,"NaN")   { carton.subtype=subtypeConst ; carton.tokVal=math.NaN() }
    }

    carton.tokType = tokType
    carton.tokText = word


get_nt_exit_point:
    // you have to set carton.tokType + startNextTokenAt by hand if you jump directly to this exit point.

    if startNextTokenAt>=lenInput { eof=true }

    rv=&lcstruct{carton,startNextTokenAt,eol,eof,borpos}
    // pf("      ... lex exit with : %#v\n",rv)
    return

}

