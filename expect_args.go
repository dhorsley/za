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
    var p int
    var type_errs string

    for v:=0; v<variants; v++ {

        nc,err:=strconv.Atoi(types[next])
        if nc==0 || len(args)!=nc {
            next=next+nc
            continue
        }
        if err!=nil { return false,errors.New(sf("internal error in %s",name)) }

        next++
        tryNext=false
        n:=0
        for p=next;p<(next+nc);p++ {
            // if types[p]=="number" {
                switch args[n].(type) {
                case nil:
                    return false,nil
                case int,uint,float64,int64,uint64,uint8:
                    if types[p]=="number" { n++; continue }
                }
            // }
            if reflect.TypeOf(args[n]).String()!=types[p] && types[p]!="any" {
                type_errs+=sf("\nargument %d - %s expected (got %s)",n+1,types[p],reflect.TypeOf(args[n]).String())
                tryNext=true
                // pf("v%d ta%d no match. moving to next.\n",v,n)
                break
            }
            // pf("v%d ta%d matched.\n",v,n)
            n++
        }
        next=next+nc
        if ! tryNext { break }
    }

    if tryNext {
        return false,errors.New(sf("\nInvalid arguments in %v",name)+type_errs)
    }

    return true, nil

}

