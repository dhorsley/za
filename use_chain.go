package main

import (
    "sync"
    "slices"
)

var chainlock = &sync.RWMutex{}  // chain access lock

var uchain = make([]string,0)
var ustack = make([][]string,0)


func uc_add(s string) (bool) {
    chainlock.Lock()
    defer chainlock.Unlock()

    if slices.Contains(uchain,s) {
        return false
    }

    uchain=append(uchain,s)
    return true
}

func uc_remove(s string) (bool) {
    chainlock.Lock()
    defer chainlock.Unlock()
    for p:=0; p<len(uchain); p+=1 {
        if uchain[p]==s {
            na:=make([]string,len(uchain)-1)
            if p!=0 {
                copy(na,uchain[:p])
            }
            if p<len(uchain)-1 {
                na=append(na[:p],uchain[p+1:]...)
            }
            uchain=na
            return true
        }
    }
    return false
}

func uc_top(s string) (bool) {
    uc_remove(s)
    chainlock.Lock()
    defer chainlock.Unlock()
    na:=make([]string,0,len(uchain)+1)
    na=append(na,s)
    for p:=0; p<len(uchain); p+=1 {
        na=append(na,uchain[p])
    }
    uchain=na 
    return true
}

func uc_reset() {
    chainlock.Lock()
    defer chainlock.Unlock()
    na:=make([]string,0)
    uchain=na
}

func ucs_push() (bool) {
    chainlock.Lock()
    defer chainlock.Unlock()
    na:=make([][]string,0,len(ustack))
    na=append(na,uchain)
    for p:=0; p<len(ustack); p+=1 {
        na=append(na,ustack[p])
    }
    ustack=na
    return true
}

func ucs_pop() (bool) {
    chainlock.Lock()
    defer chainlock.Unlock()
    uchain=ustack[0]
    ustack=ustack[1:]
    return true
}

func uc_show() {
    chainlock.RLock()
    defer chainlock.RUnlock()
    pf("USE chain\n")
    if len(uchain)==0 {
        pf("[#2]Empty[#-]\n")
    }
    for p:=0; p<len(uchain); p+=1 {
        pf("[%2d] %s\n",p,uchain[p])
    }
}

func uc_match_func(s string) (string) {
    chainlock.RLock()
    defer chainlock.RUnlock()
    for p:=0; p<len(uchain); p+=1 {
        if fnlookup.lmexists(uchain[p]+"::"+s) {
            return uchain[p]
        }
    } 
    return ""
}

func uc_match_enum(s string) (string) {
    chainlock.RLock()
    globlock.RLock()
    defer globlock.RUnlock()
    defer chainlock.RUnlock()
    for p:=0; p<len(uchain); p+=1 {
        if _,found:=enum[uchain[p]+"::"+s]; found {
            return uchain[p]
        }
    } 
    return ""
}

// @todo: structmaps is completely unprotected by locks throughout the code
//          this should be corrected. will do this later, honest.

func uc_match_struct(s string) (string) {
    chainlock.RLock()
    defer chainlock.RUnlock()
    for p:=0; p<len(uchain); p+=1 {
        if _,found:=structmaps[uchain[p]+"::"+s]; found {
            return uchain[p]
        }
    } 
    return ""
}


