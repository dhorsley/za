// +build windows

package main

import (
    "net/http"
    "reflect"
    str "strings"
    "syscall"
    "fmt"
    "unsafe"
)

// accessFieldOrFunc() is kept separate for now due to the reference to the
//  *syscall.Win32FileAttributeData struct. Eventually, fileStatSys will 
//  build a local struct with only common fields between windows and unix/bsd
//  and then this func can be returned to eval_ops.go.

func (p *leparser) accessFieldOrFunc(obj any, field string) (any,bool) {

    switch obj:=obj.(type) {

    case http.Header:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field)
        return f,false

    case *syscall.Win32FileAttributeData:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field).Interface()
        return f,false

    default:

        r := reflect.ValueOf(obj)
        // isStruct:=false

        switch r.Kind() {

        case reflect.Struct:

            // isStruct=true

            // work with mutable copy as we need to make field unsafe
            // further down in switch.

            rcopy := reflect.New(r.Type()).Elem()
            rcopy.Set(r)

            // get the required struct field and make a r/w copy
            f := rcopy.FieldByName(field)

            if f.IsValid() {

                switch f.Type().Kind() {
                case reflect.String:
                    return f.String(),false
                case reflect.Bool:
                    return f.Bool(),false
                case reflect.Int:
                    return int(f.Int()),false
                case reflect.Int64:
                    return int(f.Int()),false
                case reflect.Float64:
                    return f.Float(),false
                case reflect.Uint:
                    return uint(f.Uint()),false
                case reflect.Uint8:
                    return uint8(f.Uint()),false
                case reflect.Uint64:
                    return uint64(f.Uint()),false

                case reflect.Slice:

                    f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
                    slc:=f.Slice(0,f.Len())

                    switch f.Type().Elem().Kind() {
                    case reflect.Interface,reflect.String:
                        return slc.Interface(),false
                    default:
                        return []any{},false
                    }

                case reflect.Interface:
                    return f.Interface(),false

                default:
                    f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
                    return f.Interface(),false
                }
            }
        }

        name:=field

        // check for enum membership:
        globlock.RLock()
        // pf("checking obj %#v | enum %s\n",obj,p.preprev.tokText)
        ename:=p.namespace+"::"+p.preprev.tokText
        // isFileHandle:=false
        switch obj.(type) {
        case string:
            ename=p.namespace+"::"+obj.(string)
            checkstr:=obj.(string)
            if str.Contains(checkstr,"::") {
                // pf("enum list -> %#v\n",enum)
                cpos:=str.IndexByte(checkstr,':')
                if cpos!=-1 {
                    if len(checkstr)>cpos+1 {
                        if checkstr[cpos+1]==':' {
                            ename=checkstr
                        }
                    }
                }
            }
        case pfile:
            // isFileHandle=true
        }

        en:=enum[ename]
        globlock.RUnlock()
        if en!=nil {
            return en.members[name],false
        }

        // try a function call..
        // lhs_v would become the first argument of func lhs_f

        var isFunc bool

        // parse the function call as module '::' funcname
        modname:="main"
        if p.peek().tokType==SYM_DoubleColon {
            p.next()
            switch p.peek().tokType {
            case Identifier:
                modname=name
                name=p.peek().tokText
            default:
                panic(fmt.Errorf("invalid name in function call '%s'",p.peek().tokText))
            }
            p.next()
        }
        if _,there:=funcmap[modname+"::"+name] ; there {
            name=modname+"::"+name
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

        // user-defined or stdlib call, exception here for file handles
        var iargs []any
        // if isFileHandle || !isStruct {
            iargs=[]any{obj}
        // }

        /*
        arg_names:=[]string{}
        argpos:=1
        */

        if p.peek().tokType==LParen {
            p.next()
            if p.peek().tokType!=RParen {
                /*
                // do not currently allow named args with UFCS-like calls.
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
                    dp,err:=p.dparse(0,false)
                    if err!=nil {
                        return nil,true
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
                    dp,err:=p.dparse(0,false)
                    if err!=nil {
                        return nil,true
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

        /*
        self:=self_s{}
        if isStruct {
            self.aware=true
            self.ptr=&obj
        }
        */

        // return callFunctionExt(p.fs,p.ident,name,self,[]string{},iargs)
        return callFunctionExt(p.fs,p.ident,name,[]string{},iargs)

    }

    return nil,false
}

