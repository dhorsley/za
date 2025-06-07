
package main

import (
    "reflect"
)

func struct_match(obj any) (name string, count int) {
    obj_struct_fields := make(map[string]string, 4)
    val := reflect.ValueOf(obj)
    for i := 0; i < val.NumField(); i++ {
        n := val.Type().Field(i).Name
        t := val.Type().Field(i).Type
        obj_struct_fields[n] = t.String()
    }

    for n, structvalues := range structmaps {
        if val.NumField() != len(structvalues)/4 {
            continue
        }

        sm_struct_fields := make(map[string]string, 4)
        for svpos := 0; svpos < len(structvalues); svpos += 4 {
            pfieldtype := structvalues[svpos+1].(string)
            if pfieldtype == "float" {
                pfieldtype = "float64"
            }
            sm_struct_fields[structvalues[svpos].(string)] = pfieldtype
        }

        structs_equal := true
        for k, v := range sm_struct_fields {
            if obj_v, exists := obj_struct_fields[k]; exists {
                if v != obj_v {
                    structs_equal = false
                    break
                }
            } else {
                structs_equal = false
                break
            }
        }

        if structs_equal {
            count += 1
            name = n
        }
    }
    return
}

