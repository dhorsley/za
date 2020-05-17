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
	if lockSafety { u.Lock() }
    u.nmap[k] = v
	if lockSafety { u.Unlock() }
}

func (u *Nmap) lmget(k uint64) (string,bool) {
    var tmp string
    var ok bool
	if lockSafety { u.RLock() }
    if tmp,ok=u.nmap[k]; ok {
        if lockSafety { u.RUnlock() }
        return tmp,true
    }
    if lockSafety { u.RUnlock() }
    return "",false
}

func (u *Nmap) lmdelete(k uint64) bool {
	if lockSafety { u.Lock() }
    if _,ok:=u.nmap[k]; ok {
        delete(u.nmap,k)
	    if lockSafety { u.Unlock() }
        return true
    }
	if lockSafety { u.Unlock() }
    return false
}


