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
    // used bool
    name string
    fs uint32
    res uint64
}

var lru_bind_cache [sz_lru_cache]lru_bind

func add_lru_bind_cache(fs uint32,name string,res uint64) {
    for e:=sz_lru_cache-1; e>0; e-=1 {
        lru_bind_cache[e]=lru_bind_cache[e-1]
    }
    lru_bind_cache[0]=lru_bind{fs:fs,name:name,res:res} // ,used:true}
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

    for e:=range lru_bind_cache {
        // if lru_bind_cache[e].used==false { break }
        // fmt.Printf("cache entry [%d] -> %+v\n",e,lru_bind_cache[e])
        if fs==lru_bind_cache[e].fs {
            if strcmp(name,lru_bind_cache[e].name) {
                return lru_bind_cache[e].res
            }
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
            // pf("[parse] Discarding comment : '%+v'\n",tempToken.carton.tokText)
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

            // fmt.Printf("\nCurrent phrase: %+v\n",phrase)

            // -- discard empty lines, add phrase to func store
            if phrase.TokenCount!=0 {
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

    return badword, eof

}

