// +build !windows

package main

import (
    "net/http"
    "reflect"
    str "strings"
    "syscall"
    "unsafe"
)

// accessFieldOrFunc() is kept separate for now due to the *syscall.Stat_t
//  reference. eventually, fileStatSys will build a local struct with only
//  common fields between windows and unix/bsd and then this func can be
//  returned to eval_ops.go.

func (p *leparser) accessFieldOrFunc(obj any, field string) (any,bool) {

    // pf(" (afof) -> assessing obj %+v field %s\n",obj,field)

    switch obj:=obj.(type) {

    case http.Header:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field)
        return f,false

    case *syscall.Stat_t:
        r := reflect.ValueOf(obj)
        f := reflect.Indirect(r).FieldByName(field).Interface()
        return f,false

    default:

        // rt:= reflect.TypeOf(obj)
        r := reflect.ValueOf(obj)
        isStruct:=false
        
        switch r.Kind() {

        case reflect.Struct:

            isStruct=true
            // pf("     -> is struct\n")
            // pf("      > field : [%v]\n",field)

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
                parser.hard_fault=true
                pf("invalid name in function call '%s'\n",p.peek().tokText)
                return nil,true
            }
            p.next()
        }

        var fm Funcdef
        var there bool
        if fm,there=funcmap[modname+"::"+name] ; there {
            name=modname+"::"+name
            isFunc=true
        }

        calling_method:=false
        if isStruct {
            // compare types between (obj) and (parent)

            if fm.parent != "" {
                obj_struct_fields:=make(map[string]string,4)
                //pln("Object Field Details:")
                val := reflect.ValueOf(obj)
                for i:=0; i<val.NumField();i++ {
                    n:=val.Type().Field(i).Name
                    t:=val.Type().Field(i).Type
                    obj_struct_fields[n]=t.String()
                }

                // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value
                par_struct_fields:=make(map[string]string,4)
                if structvalues,exists:=structmaps[fm.parent] ; exists {
                    for svpos:=0; svpos<len(structvalues); svpos+=4 {
                        pfieldtype:=structvalues[svpos+1].(string)
                        if pfieldtype=="float" {
                            pfieldtype="float64"
                        }
                        // pf("parent field %2d : %v : %v\n",svpos,structvalues[svpos],pfieldtype)
                        par_struct_fields[structvalues[svpos].(string)]=pfieldtype
                    }
                }

                structs_equal:=true
                for k,v:=range par_struct_fields {
                    if obj_v,exists:=obj_struct_fields[k] ; exists {
                        if v!=obj_v {
                            structs_equal=false
                            break
                        }
                    } else {
                        structs_equal=false
                        break
                    }
                }

                // pf("parent : %v  object type : %v  :  equal? %v\n",fm.parent,rt,structs_equal)
                if ! structs_equal {
                    parser.hard_fault=true
                    pf("cannot call function [%v] belonging to an unequal struct type [%s]\nYour object: [%T]", field,fm.parent,obj)
                    return nil,true
                }
                calling_method=true
            }
        }

        // check if stdlib or user-defined function
        if !isFunc {
            if _, isFunc = stdlib[name]; !isFunc {
                isFunc = fnlookup.lmexists(name)
            }
        }

        if !isFunc {
            parser.hard_fault=true
            pf("no function, enum or record field found for %v\n", field)
            return nil,true
        }

        // user-defined or stdlib call, exception here for file handles
        var iargs []any
        iargs=[]any{obj}

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
                    dp,err,_:=p.dparse(0,false)
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

        // find caller_name if calling_method true
        res,err,method_result:=p.callFunctionExt(p.fs,p.ident,name,calling_method,obj,[]string{},iargs)

        if calling_method && !err {
            // check if previous is an identifer/expression result
            if p.preprev.tokType==Identifier {
                bin:=p.preprev.bindpos
                if (*p.ident)[bin].declared {
                    vset(nil, p.fs, p.ident, p.preprev.tokText, method_result)
                } else {
                    parser.hard_fault=true
                    pf("struct [%s] could not be assigned to after method call\n",p.preprev.tokText)
                    return nil,true
                }
            } // else { no action required as invoker was not a struct name }
        }

        return res,err
    }
}

