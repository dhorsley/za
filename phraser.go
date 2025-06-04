package main

import (
    str "strings"
    "sync"
    "time"
    "fmt"
)

// global binding list - populated during phrasing
var bindings = make([]map[string]uint64,SPACE_CAP)
var bindlock = &sync.RWMutex{}

func bindResize() {
    newar:=make([]map[string]uint64,cap(bindings)*2)
    copy(newar,bindings)
    bindings=newar
}


func bind_int(fs uint32,name string) (i uint64) {

    // fmt.Printf("Bind request for %s (fs:%d)\n",name,fs)

    bindlock.Lock()

    if bindings[fs]==nil {
        bindings[fs]=make(map[string]uint64)
        // fmt.Printf("** CLEANED BINDINGS FOR FS %d\n",fs)
    }

    var present bool
    i,present=bindings[fs][name]
    if present {
        // fmt.Printf("present @ %d\n",i)
        bindlock.Unlock()
        return
    }

    // assign if unused:
    loop:=true
    i=uint64(len(bindings[fs]))
    for ; loop ; {
        loop=false
        for _,vp:=range bindings[fs] {
            if vp==i {
                i+=1
                loop=true
                break
            }
        }
        if !loop { break }
    }

    bindings[fs][name]=i
    // fmt.Printf("new binding @ %d\n",i)
    bindlock.Unlock()
    return
}


func getFileFromIFS(ifs uint32) (string) {
    return fileMap[ifs]
}


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

    startTime:=time.Now()

    pos := start
    lstart := start

    var tempToken *lcstruct
    phrase := Phrase{
        Tokens: make([]Token,0,8),
    }
    var base   = BaseCode{}

    tokenType := Error
    curLine := int16(0)

    // simple handler for parens nesting
    var braceNestLevel  int     // round braces
    var sbraceNestLevel int     // square braces
    var defNest int             // C_Define nesting

    lmv,_:=fnlookup.lmget(fs)
    isSource[lmv]=true

    fspacelock.Lock()
    functionspaces[lmv] = make([]Phrase,0,8)
    basecode[lmv]=make([]BaseCode,0,8)
    fspacelock.Unlock()

    addToPhrase:=false
    vref_found:=false

    assert_found:=false
    on_found:=false
    do_found:=false
    borpos:=-1
    discard_phrase:=false

    lastTokenType:=Error

    for ; pos < len(input); {

        if tempToken!=nil {
            lastTokenType = tempToken.carton.tokType
        }

        tempToken = nextToken(input, lmv, &curLine, pos)
        eof=tempToken.eof

        if on_found && do_found || ! (on_found || do_found) {
            if tempToken.borpos>borpos && borpos == -1 { borpos=tempToken.borpos }
        }

        // If we found something then move the cursor along to next word
        if tempToken.tokPos != -1 { pos = tempToken.tokPos }

        tokenType = tempToken.carton.tokType

        // var_refs display
        if var_refs && tokenType==Identifier {
            if tempToken.carton.tokText==var_refs_name {
                vref_found=true
            }
        }

        // remove asserts?
        if !assert_found && tokenType==C_Assert && !enableAsserts {
            discard_phrase=true
            assert_found=true
        }

        // ON present?
        if !on_found && tokenType==C_On {
            on_found=true
        }

        // DO present?
        if !do_found && tokenType==C_Do {
            do_found=true
        }

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

        // @note: this may trip up if these tokens are beyond 
        // position 0, but not had any issues yet:
        switch tokenType {
        case C_Define:
            defNest+=1
        case C_Enddef:
            defNest-=1
        case LParen:
            braceNestLevel+=1
        case RParen:
            braceNestLevel-=1
        case LeftSBrace:
            sbraceNestLevel+=1
        case RightSBrace:
            sbraceNestLevel-=1
        }

        if sbraceNestLevel>0 || braceNestLevel>0 {
            if tempToken.eol || tokenType==EOL {
                curLine+=1
                continue
            }
        }

        // handle end-of-line dot character continuation.
        // we check borpos to ensure we are not inside a | statement also.
        // this is just meant to catch using . operator in Za multi-line expressions:
        if borpos==-1 && !permit_cmd_fallback && tempToken.eol && lastTokenType==SYM_DOT {
            // pf("eol-dot @ line %d\n",curLine+1)
            curLine+=1
            continue
        }


        if tokenType == Error {
            fmt.Printf("Error found on line %d in %s\n", curLine+1, tempToken.carton.tokText)
            break
        }

        addToPhrase = true

        if tokenType==SingleComment {
            // at this point we have returned the full comment so throw it away!
            // fmt.Printf("[parse] Discarding comment : '%+v'\n",tempToken.carton.tokText)
            addToPhrase=false
        }

        if tokenType==SYM_Semicolon || tokenType==EOL { // ditto
            addToPhrase=false
        }

        if addToPhrase {
            phrase.Tokens = append(phrase.Tokens, tempToken.carton)
            phrase.TokenCount+=1
        }

        if tokenType == EOL || tokenType == SYM_Semicolon {

            // -- add original version
            if pos>0 {
                if phrase.TokenCount>0 {
                    base.Original=input[lstart:pos]
                    if borpos>=0 {
                        base.borcmd=input[borpos:pos]
                    }
                    if tempToken.carton.tokType == EOL {
                        base.Original=base.Original[:pos-lstart-1]
                    }

                } else {
                    base.Original=""
                }
                // pf(".Original -> ·%s·\n",base.Original)
            }

            if vref_found {
                pf("[#3]%s[#-] | Line [#6]%4d[#-] : %s\n",getFileFromIFS(lmv),curLine+1,str.TrimLeft(base.Original," \t"))
                vref_found=false
            }

            phrase.SourceLine=curLine
            lstart = pos

            if tokenType==EOL { curLine+=1 }

            // fmt.Printf("\nCurrent phrase: %+v\n",phrase)

            // -- discard empty lines, add phrase to func store
            if phrase.TokenCount!=0 {
                if !discard_phrase {
                    // -- add phrase to function
                    // pf("\n[#4]for phrase text : %v\n",phrase.Tokens)
                    // pf("\n[#6]adding phrase (in #%d): %#v[#-]\n",lmv,phrase)
                    fspacelock.Lock()
                    functionspaces[lmv] = append(functionspaces[lmv], phrase)
                    basecode[lmv]       = append(basecode[lmv], base)
                    fspacelock.Unlock()
                }
            }

            // reset phrase
            phrase        = Phrase{}
            base          = BaseCode{}
            borpos        = -1
            do_found      = false
            on_found      = false
            assert_found  = false
            discard_phrase= false

        }

        if eof {
            break
        }

    }

    recordPhase([]string{fs},"parse",time.Since(startTime))

    return badword, eof

}

