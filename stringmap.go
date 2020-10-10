package main

import (
    "sync"
)

type Lmap struct {
    sync.RWMutex
    smap    map[string]uint32
}

func lmcreate(sz int) *Lmap {
    return &Lmap{smap:make(map[string]uint32,sz)}
}

func (u *Lmap) lmset(k string,v uint32) {
	u.Lock()
    u.smap[k] = v
	u.Unlock()
}

func (u *Lmap) lmget(k string) (tmp uint32,ok bool) {
	if lockSafety { u.RLock() }
    if tmp,ok=u.smap[k]; ok {
	    if lockSafety { u.RUnlock() }
        return tmp,true
    }
	if lockSafety { u.RUnlock() }
    return 0,false
}

func (u *Lmap) lmdelete(k string) bool {
	u.Lock()
    if _,ok:=u.smap[k]; ok {
        delete(u.smap,k)
        u.Unlock()
        return true
    }
    u.Unlock()
    return false
}


