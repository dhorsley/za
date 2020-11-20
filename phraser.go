package main

import (
//    str "strings"
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

    var eol bool

    var tempToken Token
    var phrase = Phrase{}

    tokenType := Error
    curLine := 0

    // simple handler for parens nesting
    var braceNestLevel  int     // round braces
    var sbraceNestLevel int     // square braces
    var tokPos int

    lmv,_:=fnlookup.lmget(fs)

    // pf("[parser] Placed source in store %d for fs '%s'\n",lmv,fs)
    // sourceStore[lmv]=make([]string,0,40)
    addToPhrase:=false

    for ; pos < len(input); {

        tempToken, tokPos, eol, eof = nextToken(input, &curLine, pos, tokenType)
        // pf(" -( parser : %s,%d,eol:%v,eof:%v )- \n",tokNames[tempToken.tokType],tokPos,eol,eof)

        // If we found something then move the cursor along to next word
        if tokPos != -1 { pos = tokPos }

        tokenType = tempToken.tokType

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
            if eol || tokenType==EOL {
                curLine++
                continue
            }
        }

        // debug(15,"nt-t: (tokpos:%d) %v\n",tokPos,tokNames[tempToken.tokType])

        if tempToken.tokType == Error {
            pf("Error found on line %d in %s\n", curLine+1, tempToken.tokText)
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
            phrase.Tokens = append(phrase.Tokens, tempToken)
            phrase.TokenCount++
        }

        if tempToken.tokType == EOL || tempToken.tokType==SYM_Semicolon {

            // -- add original version
            if pos>0 {
                if phrase.TokenCount>0 {
                    phrase.Original=input[lstart:pos]
                } else {
                        phrase.Original=""
                }

                // to be removed?
                /*
                    switch phrase.Tokens[0].tokType {
                    case C_On,SYM_BOR:
                        phrase.Original=input[lstart:pos]
                    }
                }
                */
                /*
                if phrase.TokenCount>1 {
                    switch phrase.Tokens[1].tokType {
                    case O_AssCommand:
                    //    phrase.Original=input[lstart:pos]
                    }
                }
                */

            }

            /*
            // -- add to source store
            if tempToken.tokType!=SYM_Semicolon {
                sourceStore[lmv]=append(sourceStore[lmv],str.TrimRight(input[lstart:pos]," \t\n"))
            }
            */

            phrase.SourceLine=curLine
            lstart = pos

            if tempToken.tokType==EOL { curLine++ }

            // -- discard empty lines
            if phrase.TokenCount!=0 {
                // -- add phrase to function
                fspacelock.Lock()
                functionspaces[lmv] = append(functionspaces[lmv], phrase)
                fspacelock.Unlock()
            }

            // reset phrase
            phrase = Phrase{}

        }

        if eof {
            break
        }

    }

    return badword, eof

}
