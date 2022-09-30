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

func (u *Lmap) lmshow() string {
    return sf("%#v",u.smap)
}

func (u *Lmap) lmexists(k string) bool {
    u.RLock()
    if _,ok:=u.smap[k]; ok {
        u.RUnlock()
        return true
    }
    u.RUnlock()
    return false
}

func (u *Lmap) lmset(k string,v uint32) {
    u.Lock()
    u.smap[k] = v
	u.Unlock()
}

func (u *Lmap) lmget(k string) (tmp uint32,ok bool) {
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


