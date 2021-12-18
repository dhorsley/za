package main

import (
    str "strings"
    "sync"
    "sync/atomic"
)

// global binding list - populated during phrasing
var bindings = make([]map[string]uint64,SPACE_CAP)

func bindResize() {
    newar:=make([]map[string]uint64,cap(bindings)*2)
    copy(newar,bindings)
    bindings=newar
}

// @catwalk: var ;p/09<F7>

type lru_bind struct {
    name string
    res uint64
    fs uint32
    used bool
}

var lru_bind_cache [sz_lru_cache]lru_bind

func add_lru_bind_cache(fs uint32,name string,res uint64) {
    copy(lru_bind_cache[1:],lru_bind_cache[:sz_lru_cache-1])
    lru_bind_cache[0]=lru_bind{fs:fs,name:name,res:res,used:true}
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

    var locked bool
    if atomic.LoadInt32(&concurrent_funcs)>0 { bindlock.Lock() ; locked=true }

    for e:=range lru_bind_cache {
        if lru_bind_cache[e].used==false { break }
        // fmt.Printf("cache entry [%d] -> %+v\n",e,lru_bind_cache[e])
        if fs==lru_bind_cache[e].fs {
            if strcmp(name,lru_bind_cache[e].name) {
                // fmt.Printf("[%d] %s -> found in lru cache\n",fs,name)
                if locked { bindlock.Unlock() }
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
        if locked { bindlock.Unlock() }
        return
    }

    if fs>=uint32(cap(bindings)) {
        bindResize()
    }

    i=uint64(len(bindings[fs]))
    bindings[fs][name]=i
    add_lru_bind_cache(fs,name,i)
    // fmt.Printf("[bi] added binding in #%d for %s to %d\n",fs,name,i)
    if locked { bindlock.Unlock() }
    return
}

func getFileFromIFS(ifs uint32) (string) {
    if ifs==1 { return "main" }
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
    vref_found:=false

    on_found:=false
    do_found:=false
    borpos:=-1

    for ; pos < len(input); {

        tempToken = nextToken(input, &curLine, pos)
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
            phrase.TokenCount+=1
        }

        if tokenType == EOL || tokenType == SYM_Semicolon {

            // -- add original version
            if pos>0 {
                if phrase.TokenCount>0 {
                    base.Original=input[lstart:pos]
                    if borpos>=0 {
                        base.borcmd=input[borpos:pos]
                        // base.borcmd=str.Replace(base.borcmd, `\\`, "\\", -1)
                        /*
                        pf("borcmd found @ %d\n",borpos)
                        pf("start        @ %d\n",lstart)
                        pf("in from start -> %s\n",input[lstart:pos])
                        pf("borcmd -> ·%s·\n",base.borcmd)
                        */
                    }
                    // if tempToken.carton.tokType == EOL { base.Original=base.Original[:pos-lstart-1] }
                    if tempToken.carton.tokType == EOL {
                        base.Original=base.Original[:pos-lstart-1]
                        // base.Original=str.Replace(base.Original, `\\`, "\\", -1)
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
            borpos = -1
            do_found = false
            on_found = false

        }

        if eof {
            break
        }

    }

    return badword, eof

}

