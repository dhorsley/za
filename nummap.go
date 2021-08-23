package main

import (
//    "sync"
	. "github.com/puzpuzpuz/xsync"
)

type Nmap struct {
    // sync.RWMutex
    RBMutex
    nmap    map[uint32]string
}

func nlmcreate(sz int) *Nmap {
    return &Nmap{nmap:make(map[uint32]string,sz)}
}

func (u *Nmap) lmexists(k uint32) bool {
    tk:=u.RLock()
    if _,ok:=u.nmap[k]; ok {
        u.RUnlock(tk)
        return true
    }
    u.RUnlock(tk)
    return false
}

func (u *Nmap) lmset(k uint32,v string) {
	u.Lock()
    u.nmap[k] = v
	u.Unlock()
}

func (u *Nmap) lmget(k uint32) (tmp string,ok bool) {
    tk:=u.RLock()
    if tmp,ok=u.nmap[k]; ok {
        u.RUnlock(tk)
        return tmp,true
    }
    u.RUnlock(tk)
    return "",false
}

func (u *Nmap) lmdelete(k uint32) bool {
    u.Lock()
    if _,ok:=u.nmap[k]; ok {
        delete(u.nmap,k)
	    u.Unlock()
        return true
    }
	u.Unlock()
    return false
}


