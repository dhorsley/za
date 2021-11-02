// +build windows linux freebsd

package main

import (
    "reflect"
    "strconv"
    "errors"
)


/* expect_args()
 *  called by stdlib functions for validating parameter types from user.
 *  it adds a small performance penalty but seemed the only sane option.
*/
func expect_args(name string, args []interface{}, variants int, types... string) (bool,error) {
    next:=0
    var tryNext bool
    var triedOne bool
    var p int
    var type_errs string
    var la=len(args)

    if la==0 && variants==0 {
        return true,nil
    }

    for v:=0; v<variants; v+=1 {
        nc,err:=strconv.Atoi(types[next])
        if nc==0 && la==0 {
            return true,nil
        }

        if nc==0 || la!=nc {
            next+=nc+1
            continue
        }
        if err!=nil { return false,errors.New(sf("internal error in %s : (nc->%v,type->%s)",name,nc,types[next])) }

        triedOne=true

        next+=1
        tryNext=false
        n:=0
        for p=next;p<(next+nc);p+=1 {
            switch args[n].(type) {
            case nil:
                return false,nil
            case int,uint,float64,int64,uint64,uint8:
                if types[p]=="number" { n+=1; continue }
            }
            if reflect.TypeOf(args[n]).String()!=types[p] && types[p]!="any" {
                type_errs+=sf("\nargument %d - %s expected (got %s)",n+1,types[p],reflect.TypeOf(args[n]).String())
                tryNext=true
                break
            }
            n+=1
        }
        next+=nc
        if ! tryNext { break }
    }

    if tryNext || !triedOne {
        return false,errors.New(sf("\nInvalid arguments in %v",name)+type_errs)
    }

    return true, nil

}

