//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package main

import (
    "reflect"
    "syscall"
)

func accessSyscallStatField(obj any, field string) (any, bool) {
    if s, ok := obj.(*syscall.Stat_t); ok {
        r := reflect.ValueOf(s)
        f := reflect.Indirect(r).FieldByName(field).Interface()
        return f, true
    }
    return nil, false
}

