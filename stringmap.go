package main

import (
    "sync"
)

type Lmap struct {
    sync.RWMutex
    smap    map[string]uint64
}

func lmcreate(sz int) *Lmap {
    return &Lmap{smap:make(map[string]uint64,sz)}
}

func (u *Lmap) lmset(k string,v uint64) {
	if lockSafety { u.Lock() }
    u.smap[k] = v
	if lockSafety { u.Unlock() }
}

func (u *Lmap) lmget(k string) (uint64,bool) {
    var tmp uint64
    var ok bool
	if lockSafety { u.RLock() }
    if tmp,ok=u.smap[k]; ok {
	    if lockSafety { u.RUnlock() }
        return tmp,true
    }
	if lockSafety { u.RUnlock() }
    return 0,false
}

func (u *Lmap) lmdelete(k string) bool {
	if lockSafety { u.Lock() }
    if _,ok:=u.smap[k]; ok {
        delete(u.smap,k)
        if lockSafety { u.Unlock() }
        return true
    }
    if lockSafety { u.Unlock() }
    return false
}


