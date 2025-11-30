//go:build windows

package main

import (
    "reflect"
    "syscall"
)

func accessSyscallStatField(obj any, field string) (any, bool) {
    if s, ok := obj.(*syscall.Win32FileAttributeData); ok {
        r := reflect.ValueOf(s)
        f := reflect.Indirect(r).FieldByName(field).Interface()
        return f, true
    }
    return nil, false
}
