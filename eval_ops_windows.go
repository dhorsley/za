// +build windows

package main

import (
    "net/http"
    "reflect"
    "syscall"
    "fmt"
    "unsafe"
)

// accessFieldOrFunc() is kept separate for now due to the reference to the
//  *syscall.Win32FileAttributeData struct. Eventually, fileStatSys will 
//  build a local struct with only common fields between windows and unix/bsd
//  and then this func can be returned to eval_ops.go.

func (p *leparser) accessFieldOrFunc(obj any, field string) (any) {

    switch obj:=obj.(type) {

    case http.Header:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field)
        return f

    case *syscall.Win32FileAttributeData:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field).Interface()
        return f

    default:

        r := reflect.ValueOf(obj)

        switch r.Kind() {

        case reflect.Struct:

            // work with mutable copy as we need to make field unsafe
            // further down in switch.

            rcopy := reflect.New(r.Type()).Elem()
            rcopy.Set(r)

            // get the required struct field and make a r/w copy
            f := rcopy.FieldByName(field)

            if f.IsValid() {

                switch f.Type().Kind() {
                case reflect.String:
                    return f.String()
                case reflect.Bool:
                    return f.Bool()
                case reflect.Int:
                    return int(f.Int())
                case reflect.Int64:
                    return int(f.Int())
                case reflect.Float64:
                    return f.Float()
                case reflect.Uint:
                    return uint(f.Uint())
                case reflect.Uint8:
                    return uint8(f.Uint())
                case reflect.Uint64:
                    return uint64(f.Uint())

                case reflect.Slice:

                    f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
                    slc:=f.Slice(0,f.Len())

                    switch f.Type().Elem().Kind() {
                    case reflect.Interface,reflect.String:
                        return slc.Interface()
                    default:
                        return []any{}
                    }    

                case reflect.Interface:
                    return f.Interface()

                default:
                    f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
                    return f.Interface()
                }    
            }    

        }    

        name:=field

        // check for enum membership:
        globlock.RLock()
        en:=enum[p.preprev.tokText]
        globlock.RUnlock()
        if en!=nil {
            return en.members[name]
        }

        // try a function call..
        // lhs_v would become the first argument of func lhs_f

        var isFunc bool

        // parse the function call as module '.' funcname
        nonlocal:=false
        var there bool

        if _,there=funcmap[p.preprev.tokText+"."+name] ; there {
            nonlocal=true
            name=p.preprev.tokText+"."+name
            isFunc=true
        }

        // check if stdlib or user-defined function
        if !isFunc {
            if _, isFunc = stdlib[name]; !isFunc {
                isFunc = fnlookup.lmexists(name)
            }
        }

        if !isFunc {
            panic(fmt.Errorf("no function, enum or record field found for %v", field))
        }

        // user-defined or stdlib call 

        var iargs []any
        if !nonlocal {
            iargs=[]any{obj}
        }

        /*
        arg_names:=[]string{}
        argpos:=1
        */

        if p.peek().tokType==LParen {
            p.next()
            if p.peek().tokType!=RParen {
                /*
                for {
                    switch p.peek().tokType {
                    case SYM_DOT:
                        p.next() // move-to-dot
                        p.next() // skip-to-name-from-dot
                        arg_names=append(arg_names,p.tokens[p.pos].tokText) // add name field
                    case RParen,O_Comma:
                        // missing/blank arg in list
                        panic(fmt.Errorf("missing argument #%d",argpos))
                    }
                    dp,err:=p.dparse(0)
                    if err!=nil {
                        return nil
                    }
                    iargs=append(iargs,dp)
                    if p.peek().tokType!=O_Comma {
                        break
                    }
                    p.next()
                    argpos+=1
                }
                */
                for {
                    dp,err:=p.dparse(0)
                    if err!=nil {
                        return nil
                    }
                    iargs=append(iargs,dp)
                    if p.peek().tokType!=O_Comma {
                        break
                    }
                    p.next()
                }
            }
            if p.peek().tokType==RParen {
                p.next() // consume rparen 
            }
        }

        return callFunction(p.fs,p.ident,name,arg_names,iargs)

    }

    return nil
}

