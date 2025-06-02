package main

import (
    "sync"
    )

type Lmap struct {
    m sync.Map // map[string]uint32
}

func lmcreate(sz int) *Lmap {
    return &Lmap{}
}

func (u *Lmap) lmshow() string {
    return sf("%#v",u.m)
}

func (u *Lmap) lmexists(k string) bool {
    _,ok:=u.m.Load(k)
    return ok
}

func (u *Lmap) lmset(k string,v uint32) {
    u.m.Store(k,v)
}

func (u *Lmap) lmget(k string) (tmp uint32,ok bool) {
    if v,ok:=u.m.Load(k); ok {
        return v.(uint32), true
    }
    return 0,false
}

func (u *Lmap) lmdelete(k string) bool {
    _,loaded:=u.m.LoadAndDelete(k)
    return loaded
}


