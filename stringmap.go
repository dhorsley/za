package main

import (
//    "sync"
	. "github.com/puzpuzpuz/xsync"
    )

type Lmap struct {
    // sync.RWMutex
    RBMutex
    smap    map[string]uint32
}

func lmcreate(sz int) *Lmap {
    return &Lmap{smap:make(map[string]uint32,sz)}
}

func (u *Lmap) lmexists(k string) bool {
    tk:=u.RLock()
    if _,ok:=u.smap[k]; ok {
        u.RUnlock(tk)
        return true
    }
    u.RUnlock(tk)
    return false
}

func (u *Lmap) lmset(k string,v uint32) {
    u.Lock()
    u.smap[k] = v
	u.Unlock()
}

func (u *Lmap) lmget(k string) (tmp uint32,ok bool) {
    tk:=u.RLock()
    if tmp,ok=u.smap[k]; ok {
	    u.RUnlock(tk)
        return tmp,true
    }
	u.RUnlock(tk)
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


