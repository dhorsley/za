package main

import (
//    str "strings"
//    "fmt"
)

// phraseParse():
//
//   process an input string into separate lines of commands (Phrases). Each phrase is
//   built from successive calls to nextToken(). Input ends at end-of-string or earlier
//   if an invalid token is found.
//
//   Each phrase is appended to the 'function space' (i.e. function body) of the function
//   referenced by fs. A phrase is a list of processed tokens.
//
//   functionspaces[] is a global.
//


func phraseParse(fs string, input string, start int) (badword bool, eof bool) {

    pos := start
    lstart := start

    var tempToken *lcstruct
    var phrase = Phrase{}
    var base   = BaseCode{}

    tokenType := Error
    curLine := int16(0)

    // simple handler for parens nesting
    var braceNestLevel  int     // round braces
    var sbraceNestLevel int     // square braces

    lmv,_:=fnlookup.lmget(fs)

    addToPhrase:=false

    for ; pos < len(input); {

        tempToken = nextToken(input, &curLine, pos)
        eof=tempToken.eof

        // If we found something then move the cursor along to next word
        if tempToken.tokPos != -1 { pos = tempToken.tokPos }

        tokenType = tempToken.carton.tokType

        // function name token mangling:
        if phrase.TokenCount>0 {
            if tokenType == LParen {
                prevText := phrase.Tokens[phrase.TokenCount-1].tokText
                if _, isFunc := stdlib[prevText]; !isFunc {
                    if fnlookup.lmexists(prevText) {
                        phrase.Tokens[phrase.TokenCount-1].subtype=subtypeUser
                    }
                } else {
                    phrase.Tokens[phrase.TokenCount-1].subtype=subtypeStandard
                }
            }
        }

        if tokenType==LParen {
            braceNestLevel++
        }
        if tokenType==RParen {
            braceNestLevel--
        }
        if tokenType==LeftSBrace {
            sbraceNestLevel++
        }
        if tokenType==RightSBrace {
            sbraceNestLevel--
        }

        if sbraceNestLevel>0 || braceNestLevel>0 {
            if tempToken.eol || tokenType==EOL {
                curLine++
                continue
            }
        }

        if tokenType == Error {
            pf("Error found on line %d in %s\n", curLine+1, tempToken.carton.tokText)
            break
        }

        addToPhrase = true

        if tokenType==SingleComment {
            // at this point we have returned the full comment so throw it away!
            addToPhrase=false
        }

        if tokenType==SYM_Semicolon || tokenType==EOL { // ditto
            addToPhrase=false
        }


        if addToPhrase {
            phrase.Tokens = append(phrase.Tokens, tempToken.carton)
            phrase.TokenCount++
        }

        if tokenType == EOL || tokenType == SYM_Semicolon {

            // -- add original version
            if pos>0 {
                if phrase.TokenCount>0 {
                    base.Original=input[lstart:pos]
                    if tempToken.carton.tokType == EOL { base.Original=base.Original[:pos-lstart-1] }
                    // fmt.Printf(">> %s <<\n",base.Original)
                } else {
                        base.Original=""
                }
            }

            phrase.SourceLine=curLine
            lstart = pos

            if tokenType==EOL { curLine++ }

            // -- discard empty lines
            if phrase.TokenCount!=0 {
                // -- add phrase to function
                fspacelock.Lock()
                functionspaces[lmv] = append(functionspaces[lmv], phrase)
                basecode[lmv]       = append(basecode[lmv], base)
                fspacelock.Unlock()
            }

            // reset phrase
            phrase = Phrase{}
            base   = BaseCode{}

        }

        if eof {
            break
        }

    }

    /* TEST CODE -- DO NOT ENABLE!!
    // raise an implicit C_Exit at end of function
    if lmv!=0 {
        fspacelock.Lock()
        phrase=Phrase{}
        if isMod {
            phrase.Tokens=[]Token{Token{tokType:C_Return}}
        } else {
            phrase.Tokens=[]Token{Token{tokType:C_Exit}}
        }
        phrase.TokenCount++
        functionspaces[lmv] = append(functionspaces[lmv], phrase)
        // pf("implicit-exit: %#v\n",functionspaces[lmv])
        fspacelock.Unlock()
    }
    */

    return badword, eof

}

