
package main

import (
	"fmt"
	"reflect"
    //	str "strings"
	"unsafe"
)


func (p *leparser) doAssign(lfs uint32, lident *[]Variable, rfs uint32, rident *[]Variable, tks []Token, expr *ExpressionCarton, eqPos int) {

	// (left)  lfs is the function space to assign to
	// (right) rfs is the function space to evaluate with (calculating indices expressions, etc)

	// split tks into assignees, splitting on commas

	doMulti := false
	for tok := range tks[:eqPos] {
		if tks[tok].tokType == O_Comma {
			doMulti = true
			break
		}
	}

	var largs = make([][]Token, 1)

	if doMulti {
		curArg := 0
		evnest := 0
		var scrap [16]Token
		scrapCount := 0
		for tok := range tks[:eqPos] {
			nt := tks[tok]
			if nt.tokType == LParen || nt.tokType == LeftSBrace {
				evnest += 1
			}
			if nt.tokType == RParen || nt.tokType == RightSBrace {
				evnest -= 1
			}
			if nt.tokType != O_Comma || evnest > 0 {
				scrap[scrapCount] = nt
				scrapCount += 1
			}
			if evnest == 0 && (tok == eqPos-1 || nt.tokType == O_Comma) {
				largs[curArg] = append(largs[curArg], scrap[:scrapCount]...)
				scrapCount = 0
				curArg += 1
				if curArg >= len(largs) {
					largs = append(largs, []Token{})
				}
			}
		}
		largs = largs[:curArg]
	} else {
		largs[0] = tks[:eqPos]
	}

	// pf("(da) largs -> %#v\n",largs)

	var results []any

	if len(largs) == 1 {
		if expr.result == nil {
			results = []any{nil}
		} else {
			results = []any{expr.result}
		}
	} else {
		// read results
		if expr.result != nil {
			switch expr.result.(type) {
			case []any:
				results = expr.result.([]any)
			case any:
				results = append(results, expr.result.(any))
			default:
				pf("unknown result type [%T] in expr box %#v\n", expr.result, expr.result)
			}
		} else {
			results = []any{nil}
		}
	}

	// figure number of l.h.s items and compare to results.
	if len(largs) > len(results) && len(results) > 1 {
		expr.errVal = fmt.Errorf("not enough values to populate assignment")
		expr.evalError = true
		return
	}

	var assignee []Token

	for assno := range largs {

		if assno > len(results)-1 {
			break
		}

		assignee = largs[assno]

		/*
		   pf("[#6]");
		   pf("assignee #%d\n",assno)
		   pf("assignee token : %#v\n",assignee)
		   pf("assignee value : %+v\n",results[assno])
		   pf("[#-]")
		*/

		if assignee[0].tokType != Identifier {
			expr.errVal = fmt.Errorf("Assignee must be an identifier (not '%s')", assignee[0].tokText)
			expr.evalError = true
			return
		}

		// ignore assignment to underscore
		if assignee[0].tokText == "_" {
			continue
		}

		// then apply the shite below to each one, using the next available result from results[]

        /*
		dotAt := -1
		rbAt := -1
		var rbSet, dotSet bool
		for dp := len(assignee) - 1; dp > 0; dp -= 1 {
			if !rbSet && assignee[dp].tokType == RightSBrace {
				rbAt = dp
				rbSet = true
			}
			if !dotSet && assignee[dp].tokType == SYM_DOT {
				dotAt = dp
				dotSet = true
			}
		}
        */

		// struct content duplication
		isStruct := false
		if results[assno] != nil {
			isStruct = reflect.TypeOf(results[assno]).Kind() == reflect.Struct
		}

		struct_name := ""
		if isStruct {
			type_string, count := struct_match(results[assno])
			if count == 1 {
				struct_name = type_string
			}
			// pf("[ev] [asslen1] struct match string [%s]\n",struct_name)
		}

		if isStruct && lfs == rfs && len(assignee) == 1 {
			bin := bind_int(lfs, assignee[0].tokText)
			assref := (*lident)[bin]
			if assref.Kind_override == "" {
				// recipient not a struct, just overwrite as usual
			} else {
				// is a struct, check type is compatible
				obj_struct_fields := make(map[string]string, 4)
				val := reflect.ValueOf(results[assno])
				for i := 0; i < val.NumField(); i++ {
					n := val.Type().Field(i).Name
					t := val.Type().Field(i).Type
					obj_struct_fields[n] = t.String()
				}
				ass_struct_fields := make(map[string]string, 4)
				val = reflect.ValueOf(assref.IValue)
				for i := 0; i < val.NumField(); i++ {
					n := val.Type().Field(i).Name
					t := val.Type().Field(i).Type
					ass_struct_fields[n] = t.String()
				}

				structs_equal := true
				if len(ass_struct_fields) != len(obj_struct_fields) {
					structs_equal = false
				}

				for k, v := range ass_struct_fields {
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
					assref.IValue = results[assno]
					assref.ITyped = false
					assref.declared = true
					assref.Kind_override = struct_name
					(*lident)[bin] = assref
				} else {
					expr.errVal = fmt.Errorf(
						"Dissimilar struct types in assignment [left:%s] [right:%s]",
						assref.Kind_override,
						reflect.TypeOf(results[assno]).Kind(),
					)
					expr.evalError = true
					return
				}
			}
		}

		switch {
		case len(assignee) == 1:
			///////////// CHECK FOR a       /////////////////////////////////////////////
			// normal assignment

			if lfs == rfs {
				vset(&assignee[0], lfs, lident, assignee[0].tokText, results[assno])
			} else {
				vset(nil, lfs, lident, assignee[0].tokText, results[assno])
				bin := bind_int(lfs, assignee[0].tokText)
				if isStruct {
					(*lident)[bin].Kind_override = struct_name
				}
			}

		default:
			// Handle nested LHS
			container, finalKey, isField, err := p.resolveLHSTarget(lfs, lident, rfs, rident, assignee)
			if err != nil {
				pf("could not evaluate index or key in assignment: %v", err)
				expr.evalError = true
				expr.errVal = err
				return
			}

			// Get the last token to determine assignment type
			lastToken := assignee[len(assignee)-1]
			switch {
			case lastToken.tokType == RightSBrace:
				// Array/map assignment
				if finalKey == nil {
					pf("invalid array/map assignment: %v", assignee[0].tokText)
					expr.errVal = fmt.Errorf("invalid array/map assignment")
					expr.evalError = true
					return
				}

				// Check if this is a dotMode assignment (a[e].f=)
				if len(assignee) > 2 && assignee[len(assignee)-2].tokType == SYM_DOT {
					// Get the original container
					container, ok := vget(&assignee[0], lfs, lident, assignee[0].tokText)
					if !ok {
						pf("variable %v not found", assignee[0].tokText)
						expr.evalError = true
						expr.errVal = fmt.Errorf("variable not found")
						return
					}

					// Create a copy of the container
					val := reflect.ValueOf(container)
					tmp := reflect.New(val.Type()).Elem()
					tmp.Set(val)

					// Get the element
					var elem reflect.Value
					if val.Kind() == reflect.Map {
						elem = tmp.MapIndex(reflect.ValueOf(finalKey))
						if !elem.IsValid() {
							pf("key not found in map: %v", finalKey)
							expr.evalError = true
							expr.errVal = fmt.Errorf("key not found")
							return
						}
					} else {
						idx, ok := finalKey.(int)
						if !ok {
							pf("array index must be integer: %v", finalKey)
							expr.evalError = true
							expr.errVal = fmt.Errorf("invalid array index")
							return
						}
						if idx < 0 || idx >= tmp.Len() {
							pf("array index out of bounds: %v", idx)
							expr.evalError = true
							expr.errVal = fmt.Errorf("index out of bounds")
							return
						}
						elem = tmp.Index(idx)
					}

					// Get the field to modify
					field := elem.FieldByName(lastToken.tokText)
					if !field.IsValid() {
						pf("field %v not found in element", lastToken.tokText)
						expr.evalError = true
						expr.errVal = fmt.Errorf("field not found")
						return
					}

					// Make the field writable and assign the new value
					field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
					field.Set(reflect.ValueOf(results[assno]))

					// Write the modified container back
					if lfs == rfs {
						vset(&assignee[0], lfs, lident, assignee[0].tokText, tmp.Interface())
					} else {
						vset(nil, lfs, lident, assignee[0].tokText, tmp.Interface())
					}
				} else {
					// Normal array/map element assignment
					vsetElement(&assignee[0], lfs, lident, assignee[0].tokText, finalKey, results[assno])
				}

			case lastToken.tokType == Identifier && len(assignee) > 1 && assignee[len(assignee)-2].tokType == SYM_DOT:
				// Struct field assignment
				if !isField {
					pf("invalid struct field assignment: %v.%v", assignee[0].tokText, lastToken.tokText)
					expr.errVal = fmt.Errorf("invalid struct field assignment")
					expr.evalError = true
					return
				}

				// Use existing struct field assignment logic with unsafe modification
				val := reflect.ValueOf(container)
				if val.Kind() != reflect.Struct {
					pf("variable %v is not a STRUCT", assignee[0].tokText)
					expr.evalError = true
					expr.errVal = fmt.Errorf("cannot assign to field of non-struct type")
					return
				}

				// Create temp copy of struct
				tmp := reflect.New(val.Type()).Elem()
				tmp.Set(val)

				field := tmp.FieldByName(lastToken.tokText)
				if !field.IsValid() {
					pf("STRUCT field %v not found in %v", lastToken.tokText, assignee[0].tokText)
					expr.evalError = true
					expr.errVal = fmt.Errorf("field %s not found", lastToken.tokText)
					return
				}

				// Handle nil assignment
				if results[assno] == nil {
					field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
					if lfs == rfs {
						vset(&assignee[0], lfs, lident, assignee[0].tokText, tmp.Interface())
					} else {
						vset(nil, lfs, lident, assignee[0].tokText, tmp.Interface())
					}
					return
				}

				// Bodge: special case assignments to coerce type:
				switch field.Type().String() {
				case "*big.Int":
					results[assno] = GetAsBigInt(results[assno])
				case "*big.Float":
					results[assno] = GetAsBigFloat(results[assno])
				}
				switch results[assno].(type) {
				case uint32:
					results[assno] = int(results[assno].(uint32))
				}
				// end-bodge

				if !reflect.ValueOf(results[assno]).Type().AssignableTo(field.Type()) {
					pf("cannot assign result (%T) to %v.%v (%v)", results[assno], assignee[0].tokText, lastToken.tokText, field.Type())
					expr.evalError = true
					expr.errVal = fmt.Errorf("cannot assign %T to field of type %v", results[assno], field.Type())
					return
				}

				// Make r/w then assign the new value into the copied field
				field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
				field.Set(reflect.ValueOf(results[assno]))

				// Write the copy back to the 'real' variable
				if lfs == rfs {
					vset(&assignee[0], lfs, lident, assignee[0].tokText, tmp.Interface())
				} else {
					vset(nil, lfs, lident, assignee[0].tokText, tmp.Interface())
				}

			default:
				pf("syntax error in assignment")
				pf(":\n->%d:%v\n", assno, assignee)
				expr.evalError = true
				expr.errVal = fmt.Errorf("invalid assignment target")
				return
			}
		}

	} // end for assno

}

// resolveLHSTarget walks the LHS token chain and returns the container and final key/index/field
func (p *leparser) resolveLHSTarget(lfs uint32, lident *[]Variable, rfs uint32, rident *[]Variable, tokens []Token) (container any, finalKey any, isField bool, err error) {
	// Get the container (array/map/struct)
	container, ok := vget(&tokens[0], lfs, lident, tokens[0].tokText)
	if !ok {
		return nil, nil, false, fmt.Errorf("variable %s not found", tokens[0].tokText)
	}

	// Handle array/map access
	if len(tokens) > 1 && tokens[1].tokType == LeftSBrace {
		// Find the matching right bracket
		rbPos := -1
		for i := 1; i < len(tokens); i++ {
			if tokens[i].tokType == RightSBrace {
				rbPos = i
				break
			}
		}
		if rbPos == -1 {
			return nil, nil, false, fmt.Errorf("missing closing bracket")
		}

		// Evaluate the index/key expression using Eval
		key, err := p.Eval(rfs, tokens[2:rbPos])
		if err != nil {
			return nil, nil, false, fmt.Errorf("invalid index/key expression: %v", err)
		}

		// Get the element
		val := reflect.ValueOf(container)
		if val.Kind() != reflect.Array && val.Kind() != reflect.Slice && val.Kind() != reflect.Map {
			return nil, nil, false, fmt.Errorf("cannot index into non-array/map type")
		}

		if val.Kind() == reflect.Map {
			elem := val.MapIndex(reflect.ValueOf(key))
			if !elem.IsValid() {
				return nil, nil, false, fmt.Errorf("key not found in map")
			}
			container = elem.Interface()
		} else {
			idx, ok := key.(int)
			if !ok {
				return nil, nil, false, fmt.Errorf("array index must be integer")
			}
			if idx < 0 || idx >= val.Len() {
				return nil, nil, false, fmt.Errorf("array index out of bounds")
			}
			container = val.Index(idx).Interface()
		}

		// If this is a dotMode assignment (a[e].f=), return the key for later use
		if len(tokens) > rbPos+1 && tokens[rbPos+1].tokType == SYM_DOT {
			finalKey = key
			return container, finalKey, false, nil
		}
	}

	// Handle struct field access
	if len(tokens) > 1 && tokens[len(tokens)-2].tokType == SYM_DOT {
		fieldName := tokens[len(tokens)-1].tokText
		val := reflect.ValueOf(container)
		if val.Kind() != reflect.Struct {
			return nil, nil, false, fmt.Errorf("cannot access field of non-struct type")
		}
		field := val.FieldByName(fieldName)
		if !field.IsValid() {
			return nil, nil, false, fmt.Errorf("field %s not found", fieldName)
		}
		return container, nil, true, nil
	}

	return container, nil, false, nil
}
