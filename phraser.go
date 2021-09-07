package main

import (
    // "fmt"
    "sync"
    "sync/atomic"
)

// global binding list - populated during phrasing
var bindings = make([]map[string]uint64,MAX_FUNCS)

func bindResize() {
    newar:=make([]map[string]uint64,cap(bindings)*2)
    copy(newar,bindings)
    bindings=newar
}

// @catwalk: var ;p/09<F7>

type lru_bind struct {
    fs uint32
    name string
    res uint64
}

var lru_bind_cache [sz_lru_cache]lru_bind

func add_lru_bind_cache(fs uint32,name string,res uint64) {
    for e:=sz_lru_cache-1; e>0; e-=1 {
        lru_bind_cache[e]=lru_bind_cache[e-1]
    }
    lru_bind_cache[0]=lru_bind{fs:fs,name:name,res:res}
}

/*
func invalidate_lru_bind_cache() {
    for e:=0; e<sz_lru_cache; e+=1 {
        lru_bind_cache[e]=lru_bind{fs:0,name:"",res:0}
    }
    // fmt.Printf("cache invalidated!\n")
}
*/

var bindlock = &sync.RWMutex{}

func bind_int(fs uint32,name string) (i uint64) {

    if atomic.LoadInt32(&concurrent_funcs)>0 { bindlock.Lock() ; defer bindlock.Unlock() }
    // bindlock.Lock() ; defer bindlock.Unlock()

    for e:=range lru_bind_cache {
        if fs==lru_bind_cache[e].fs && strcmp(name,lru_bind_cache[e].name) {
            return lru_bind_cache[e].res
        }
    }

    if bindings[fs]==nil {
        bindings[fs]=make(map[string]uint64)
    }
    var present bool
    if i,present=bindings[fs][name]; present {
        add_lru_bind_cache(fs,name,i)
        return
    }

    if fs>=uint32(cap(bindings)) {
        bindResize()
    }

    i=uint64(len(bindings[fs]))
    bindings[fs][name]=i
    add_lru_bind_cache(fs,name,i)
    // fmt.Printf("[bi] added binding in #%d for %s to %d\n",fs,name,i)
    return
}


/*
func build_bindings(fs uint32,phrase *Phrase) {
    for k,t:=range (*phrase).Tokens {
        switch t.tokType {
        case Identifier:
            i:=bind_int(fs,t.tokText)
            (*phrase).Tokens[k].sid=i
            // pf("[bind] fs %d | name %s | val %d\n",fs,t.tokText,i)
        }
    }
    // pf("Symbol Table # %d\n%+v\n",fs,bindings[fs])
}
*/


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
    var defNest int             // C_Define nesting

    lmv,_:=fnlookup.lmget(fs)

    // bindings[lmv]=make(map[string]uint64)
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

        // @note: this may trip up if these tokens are beyond 
        // position 0, but not had any issues yet:
        switch tokenType {
        case C_Define:
            defNest+=1
        case C_Enddef:
            defNest-=1
        case LParen:
            braceNestLevel++
        case RParen:
            braceNestLevel--
        case LeftSBrace:
            sbraceNestLevel++
        case RightSBrace:
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
            // pf("[parse] Discarding comment : '%+v'\n",tempToken.carton.tokText)
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

            // fmt.Printf("\nCurrent phrase: %+v\n",phrase)

            // -- discard empty lines, add phrase to func store
            if phrase.TokenCount!=0 {

                /*
                // -- generate bindings
                if defNest==0 {
                    if len(phrase.Tokens)>1 {
                        switch phrase.Tokens[0].tokType {
                        case Identifier:
                            switch phrase.Tokens[1].tokType {
                            case O_AssCommand, O_Assign:
                                // only bind first Token
                                // this potentially ignores other stuff before the =| or =
                                // but they would be bound elsewhere if they are used.
                                // phrase.Tokens[0].sid=bind_int(lmv,phrase.Tokens[0].tokText)
                                // pf("[phrase] in fs #%d, just bound %s to %d\n",lmv,phrase.Tokens[0].tokText,phrase.Tokens[0].sid)
                            }
                        case C_Async, C_Var,C_Input,C_Enum,C_For,C_Foreach:
                            // phrase.Tokens[1].sid=bind_int(lmv,phrase.Tokens[1].tokText)
                            // pf("[phrase] in fs #%d, just bound %s to %d\n",lmv,phrase.Tokens[1].tokText,phrase.Tokens[1].sid)
                        }
                    }
                }
                */

                // -- add phrase to function
                // fmt.Printf("adding phrase (in #%d): %+v\n",lmv,phrase)
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

