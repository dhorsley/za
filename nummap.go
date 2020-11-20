package main

import (
    "sync"
)

type Nmap struct {
    sync.RWMutex
    nmap    map[uint32]string
    recent  [80]uint32
}

func nlmcreate(sz int) *Nmap {
    return &Nmap{nmap:make(map[uint32]string,sz)}
}

func (u *Nmap) lmexists(k uint32) bool {

    //for e:=0; e<80; e++ {
    //    if u.recent[e]==k { return true }
    //}

    if _,ok:=u.nmap[k]; ok {
    //    copy(u.recent[:], u.recent[1:])
    //    u.recent[79]=k
        return true
    }
    return false
}

func (u *Nmap) lmset(k uint32,v string) {
	u.Lock()
    u.nmap[k] = v
	u.Unlock()
}

func (u *Nmap) lmget(k uint32) (tmp string,ok bool) {
	u.RLock()
    if tmp,ok=u.nmap[k]; ok {
        u.RUnlock()
        return tmp,true
    }
    u.RUnlock()
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


