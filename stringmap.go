package main

import (
    "sync"
)

type Lmap struct {
    sync.RWMutex
    smap    map[string]uint32
    recent  [80]string
}

func lmcreate(sz int) *Lmap {
    return &Lmap{smap:make(map[string]uint32,sz)}
}

func (u *Lmap) lmexists(k string) bool {

    //for e:=0; e<80; e++ {
    //    if u.recent[e]==k { return true }
    //}

    if _,ok:=u.smap[k]; ok {
    //    copy(u.recent[:], u.recent[1:])
    //    u.recent[79]=k
        return true
    }

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


