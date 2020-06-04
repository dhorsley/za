package main

import (
    "sync"
)

type Nmap struct {
    sync.RWMutex
    nmap    map[uint64]string
}

func nlmcreate(sz int) *Nmap {
    return &Nmap{nmap:make(map[uint64]string,sz)}
}

func (u *Nmap) lmset(k uint64,v string) {
	u.Lock()
    u.nmap[k] = v
	u.Unlock()
}

func (u *Nmap) lmget(k uint64) (string,bool) {
    var tmp string
    var ok bool
	u.RLock()
    if tmp,ok=u.nmap[k]; ok {
        u.RUnlock()
        return tmp,true
    }
    u.RUnlock()
    return "",false
}

func (u *Nmap) lmdelete(k uint64) bool {
	u.Lock()
    if _,ok:=u.nmap[k]; ok {
        delete(u.nmap,k)
	    u.Unlock()
        return true
    }
	u.Unlock()
    return false
}


