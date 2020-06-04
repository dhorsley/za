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
	u.Lock()
    u.smap[k] = v
	u.Unlock()
}

func (u *Lmap) lmget(k string) (uint64,bool) {
    var tmp uint64
    var ok bool
	u.RLock()
    if tmp,ok=u.smap[k]; ok {
	    u.RUnlock()
        return tmp,true
    }
	u.RUnlock()
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


