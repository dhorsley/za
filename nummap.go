package main

import (
    "sync"
)

type Nmap struct {
    m sync.Map // map[uint32]string
}

func nlmcreate(sz int) *Nmap {
    return &Nmap{}
}

func (u *Nmap) lmshow() string {
    return sf("%#v",u.m)
}


func (u *Nmap) lmexists(k uint32) bool {
    _, ok := u.m.Load(k)
    return ok
}

func (u *Nmap) lmset(k uint32, v string) {
    u.m.Store(k, v)
}

func (u *Nmap) lmget(k uint32) (tmp string, ok bool) {
    if v, ok := u.m.Load(k); ok {
        return v.(string), true
    }
    return "", false
}

func (u *Nmap) lmdelete(k uint32) bool {
    _, loaded := u.m.LoadAndDelete(k)
    return loaded
}


