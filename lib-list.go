//go:build !test

package main

/*

   General array and array-as-list functions.

   Let's never talk about this code.

*/

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// Helper function to process axis and keepdims parameters
func processAxisParametersList(args []any) (axis int, keepdims bool, err error) {
	axis = -1 // default: flatten all (current behavior)
	keepdims = false

	if len(args) > 0 {
		switch v := args[0].(type) {
		case int:
			axis = v
		case string:
			if v == "None" {
				axis = -1
			} else {
				return -1, false, errors.New("axis must be an integer or 'None'")
			}
		default:
			return -1, false, errors.New("axis must be an integer or 'None'")
		}
	}

	if len(args) > 1 {
		if kd, ok := args[1].(bool); ok {
			keepdims = kd
		} else {
			return -1, false, errors.New("keepdims must be a boolean")
		}
	}

	return axis, keepdims, nil
}

// Helper function to get dimensions of an array
func getArrayDimensionsList(array any) []int {
	if !isSlice(array) {
		return []int{}
	}

	var dims []int
	arr := reflect.ValueOf(array)

	for arr.Kind() == reflect.Slice {
		dims = append(dims, arr.Len())
		if arr.Len() == 0 {
			// Empty slice - can't go deeper
			break
		}
		arr = arr.Index(0)
		if arr.Kind() == reflect.Interface {
			arr = arr.Elem()
		}
	}

	return dims
}

// Helper function to apply operation along specified axis
func applyAlongAxisList(array any, axis int, operation func([]any) any) (any, error) {
	dims := getArrayDimensionsList(array)

	if axis == -1 {
		// Flatten and apply operation (current behavior)
		flat := flattenSlice(array)
		return operation(flat), nil
	}

	if axis >= len(dims) {
		return nil, fmt.Errorf("axis %d is out of bounds for array with %d dimensions", axis, len(dims))
	}

	// Handle empty arrays
	if len(dims) == 0 || (len(dims) > 0 && dims[0] == 0) {
		// Empty array - return empty result
		return []any{}, nil
	}

	// For now, implement 2D case (most common)
	if len(dims) == 2 {
		flat := flattenSlice(array)
		if axis == 0 {
			// Operate along columns (first dimension)
			result := make([]any, dims[1])
			for col := 0; col < dims[1]; col++ {
				column := make([]any, dims[0])
				for row := 0; row < dims[0]; row++ {
					// Extract element at [row][col]
					index := row*dims[1] + col
					if index < len(flat) {
						column[row] = flat[index]
					}
				}
				result[col] = operation(column)
			}
			return result, nil
		} else if axis == 1 {
			// Operate along rows (second dimension)
			result := make([]any, dims[0])
			for row := 0; row < dims[0]; row++ {
				rowData := make([]any, dims[1])
				for col := 0; col < dims[1]; col++ {
					// Extract element at [row][col]
					index := row*dims[1] + col
					if index < len(flat) {
						rowData[col] = flat[index]
					}
				}
				result[row] = operation(rowData)
			}
			return result, nil
		}
	}

	// For higher dimensions or unsupported cases, fall back to flatten
	flat := flattenSlice(array)
	return operation(flat), nil
}

// Helper function to apply keepdims
func applyKeepdimsList(result any, originalDims []int, axis int, keepdims bool) any {
	if !keepdims || axis == -1 {
		return result
	}

	// For 2D arrays, preserve the reduced dimension
	if len(originalDims) == 2 {
		if axis == 0 {
			// Column operation - result should be 1xN
			if resSlice, ok := result.([]any); ok {
				return [][]any{resSlice}
			}
		} else if axis == 1 {
			// Row operation - result should be Nx1
			if resSlice, ok := result.([]any); ok {
				resultMatrix := make([][]any, len(resSlice))
				for i, val := range resSlice {
					resultMatrix[i] = []any{val}
				}
				return resultMatrix
			}
		}
	}

	return result
}

var multiSortMu sync.Mutex

type sortStructInt struct {
	k string
	v int
}

type sortStructUint struct {
	k string
	v uint
}

type sortStructString struct {
	k string
	v string
}

type sortStructInterface struct {
	k string
	v any
}

type sortStructFloat struct {
	k string
	v float64
}

///////////////////////////////////////////////////////////////////////

// readUnexportedField uses unsafe to access an unexported field of an addressable struct.
func readUnexportedField(v reflect.Value) any {
	ptr := unsafe.Pointer(v.UnsafeAddr())
	rv := reflect.NewAt(v.Type(), ptr).Elem()
	return rv.Interface()
}

// getFieldValue extracts the field value as interface, handling private fields via unsafe.
func getFieldValue(structVal reflect.Value, fieldName string) any {
	// Try exact match first
	field := structVal.FieldByName(fieldName)
	// If not found, try capitalized variant
	if !field.IsValid() && fieldName != "" {
		field = structVal.FieldByName(strings.Title(fieldName))
	}
	if !field.IsValid() {
		return nil // field does not exist
	}

	if field.CanInterface() {
		return field.Interface()
	}
	// Fallback for unexported fields
	return readUnexportedField(field)
}

// getLessValue compares two interface values (already resolved).
func getLessValue(a, b any) bool {
	switch ai := a.(type) {
	case int:
		return ai < b.(int)
	case int8:
		return ai < b.(int8)
	case int16:
		return ai < b.(int16)
	case int32:
		return ai < b.(int32)
	case int64:
		return ai < b.(int64)
	case uint:
		return ai < b.(uint)
	case uint8:
		return ai < b.(uint8)
	case uint16:
		return ai < b.(uint16)
	case uint32:
		return ai < b.(uint32)
	case uint64:
		return ai < b.(uint64)
	case float32:
		return ai < b.(float32)
	case float64:
		return ai < b.(float64)
	case string:
		return ai < b.(string)
	case *big.Int:
		return ai.Cmp(b.(*big.Int)) < 0
	case *big.Float:
		return ai.Cmp(b.(*big.Float)) < 0
	}
	return false
}

func MultiSorted(inputSlice any, inputSortKeys []string, ascendingSortOrder []bool) ([]any, error) {
	sliceVal := reflect.ValueOf(inputSlice)
	if sliceVal.Kind() != reflect.Slice {
		return nil, errors.New("MultiSorted: inputSlice must be a slice")
	}
	if len(ascendingSortOrder) == 0 {
		ascendingSortOrder = make([]bool, len(inputSortKeys))
		for i := range ascendingSortOrder {
			ascendingSortOrder[i] = true
		}
	}
	if len(inputSortKeys) != len(ascendingSortOrder) {
		return nil, errors.New("MultiSorted: sort keys and sort orders length mismatch")
	}

	// Shallow copy
	sliceCopy := reflect.MakeSlice(sliceVal.Type(), sliceVal.Len(), sliceVal.Len())
	reflect.Copy(sliceCopy, sliceVal)

	multiSortMu.Lock()
	defer multiSortMu.Unlock()

	sort.Slice(sliceCopy.Interface(), func(i, j int) bool {
		vi := sliceCopy.Index(i)
		vj := sliceCopy.Index(j)

		// Unwrap interfaces
		if vi.Kind() == reflect.Interface {
			vi = vi.Elem()
		}
		if vj.Kind() == reflect.Interface {
			vj = vj.Elem()
		}

		// Create real, addressable struct copies (like 'tmp' in eval.go)
		tmpi := reflect.New(vi.Type()).Elem()
		tmpi.Set(vi)
		tmpj := reflect.New(vj.Type()).Elem()
		tmpj.Set(vj)

		// Compare based on priority keys
		for idx, key := range inputSortKeys {
			fi := getFieldValue(tmpi, key)
			fj := getFieldValue(tmpj, key)

			if fi == nil || fj == nil {
				continue
			}

			if getLessValue(fi, fj) {
				return ascendingSortOrder[idx]
			} else if getLessValue(fj, fi) {
				return !ascendingSortOrder[idx]
			}
			// else equal, check next key
		}

		return false
	})

	// Convert sorted copy to []any
	out := make([]any, sliceCopy.Len())
	for i := 0; i < sliceCopy.Len(); i++ {
		out[i] = sliceCopy.Index(i).Interface()
	}
	return out, nil
}

///////////////////////////////////////////////////////////////////////

func anyDissimilar(list []any) bool {
	knd := sf("%T", list[0])
	for _, v := range list[1:] {
		if sf("%T", v) != knd {
			return true
		}
	}
	return false
}

func buildNum(a string) float64 {

	// @note: deca (da) is missing. doesn't fit the scheme and is
	//   not commonly used.
	//   kilo (K) is also aliased to lower-case k for common misuse.

	var unitOrder = make(map[rune]int)
	unitOrder['K'] = 3
	unitOrder['M'] = 6
	unitOrder['G'] = 9
	unitOrder['T'] = 12
	unitOrder['P'] = 15
	unitOrder['E'] = 18
	unitOrder['Z'] = 21
	unitOrder['Y'] = 24
	unitOrder['k'] = 3
	unitOrder['h'] = 2
	unitOrder['d'] = -1
	unitOrder['c'] = -2
	unitOrder['m'] = -3
	unitOrder['u'] = -6
	unitOrder['μ'] = -6
	unitOrder['n'] = -9
	unitOrder['p'] = -12
	unitOrder['f'] = -15
	unitOrder['a'] = -18
	unitOrder['z'] = -21
	unitOrder['y'] = -24

	minus := false
	digits := ""
	unit := 0
	for p, c := range a {
		if p == 0 && c == '-' {
			minus = !minus
			continue
		}
		if c == '.' || (c >= '0' && c <= '9') {
			digits += string(c)
			continue
		}
		if _, found := unitOrder[c]; found {
			unit = unitOrder[c]
			break
		}
	}
	astr := ""
	if minus {
		astr = "-"
	}
	astr += digits
	aval, aerr := GetAsFloat(astr)
	if aerr {
		return math.NaN()
	}
	return aval * math.Pow10(unit)
}

// naive solution. should instead do similar to strnumcmp-in.h numcompare() from coreutils.
func human_numcompare(astr, bstr string) bool {
	a := buildNum(astr)
	b := buildNum(bstr)
	return a < b
}

func human_numcompare_reverse(astr, bstr string) bool {
	return human_numcompare(bstr, astr)
}

// naturalCompare compares strings with embedded numbers in natural order
func naturalCompare(a, b string) bool {
	// Split strings into parts (text and numbers)
	aParts := splitAlphanumeric(a)
	bParts := splitAlphanumeric(b)

	// Compare each part
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aPart := aParts[i]
		bPart := bParts[i]

		// If both parts are numbers, compare numerically
		if aPart.isNumber && bPart.isNumber {
			if aPart.numValue != bPart.numValue {
				return aPart.numValue < bPart.numValue
			}
		} else {
			// Otherwise compare as strings
			if aPart.text != bPart.text {
				return aPart.text < bPart.text
			}
		}
	}

	// If all parts match up to the shorter length, shorter comes first
	return len(aParts) < len(bParts)
}

type alphanumericPart struct {
	text     string
	numValue int64
	isNumber bool
}

func splitAlphanumeric(s string) []alphanumericPart {
	var parts []alphanumericPart
	var current strings.Builder
	var numStr strings.Builder
	inNumber := false

	for _, r := range s {
		if r >= '0' && r <= '9' {
			if !inNumber && current.Len() > 0 {
				// Save the text part
				parts = append(parts, alphanumericPart{text: current.String(), isNumber: false})
				current.Reset()
			}
			inNumber = true
			numStr.WriteRune(r)
		} else {
			if inNumber {
				// Save the number part
				numVal, _ := strconv.ParseInt(numStr.String(), 10, 64)
				parts = append(parts, alphanumericPart{numValue: numVal, isNumber: true})
				numStr.Reset()
				inNumber = false
			}
			current.WriteRune(r)
		}
	}

	// Handle remaining parts
	if inNumber {
		numVal, _ := strconv.ParseInt(numStr.String(), 10, 64)
		parts = append(parts, alphanumericPart{numValue: numVal, isNumber: true})
	} else if current.Len() > 0 {
		parts = append(parts, alphanumericPart{text: current.String(), isNumber: false})
	}

	return parts
}

func buildListLib() {

	features["list"] = Feature{version: 1, category: "data"}
	categories["list"] = []string{"col", "head", "tail", "sum", "fieldsort", "ssort", "sort", "uniq",
		"append", "append_to", "insert", "remove", "push_front", "pop", "peek",
		"any", "all", "esplit", "min", "max", "avg", "eqlen",
		"empty", "list_string", "list_float", "list_int", "list_int64", "list_bool", "list_bigi", "list_bigf",
		"scan_left", "zip", "list_fill", "concat",
	}

	slhelp["scan_left"] = LibHelp{in: "numeric_list,op_string,start_seed", out: "list", action: "Creates a list from the intermediary values of processing [#i1]op_string[#i0] while iterating over [#i1]list[#i0]."}
	stdlib["scan_left"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("scan_left", args, 3,
			"3", "[]int", "string", "number",
			"3", "[]float64", "string", "number",
			"3", "[]interface {}", "string", "number"); !ok {
			return nil, err
		}

		op_string := args[1].(string)

		var reduceparser *leparser
		reduceparser = &leparser{}
		// calllock.RLock()
		reduceparser.ident = ident
		reduceparser.fs = evalfs
		reduceparser.ctx = withProfilerContext(context.Background())
		// calllock.RUnlock()

		switch args[0].(type) {
		case []int:
			var seed int
			switch args[2].(type) {
			case int:
				seed = args[2].(int)
			default:
				return nil, errors.New("seed must be an int")
			}
			var new_list []int
			for q := range args[0].([]int) {
				expr := strconv.Itoa(seed) + op_string + strconv.Itoa(args[0].([]int)[q])
				res, err := ev(reduceparser, evalfs, expr)
				if err != nil {
					return nil, errors.New("could not process list")
				}
				seed = res.(int)
				new_list = append(new_list, res.(int))
			}
			return new_list, nil

		case []float64:
			var seed float64
			switch args[2].(type) {
			case float64:
				seed = args[2].(float64)
			default:
				return nil, errors.New("seed must be a float64")
			}
			var new_list []float64
			for q := range args[0].([]float64) {
				expr := strconv.FormatFloat(seed, 'f', -1, 64) + op_string + strconv.FormatFloat(args[0].([]float64)[q], 'f', -1, 64)
				res, err := ev(reduceparser, evalfs, expr)
				if err != nil {
					return nil, errors.New("could not process list")
				}
				seed = res.(float64)
				new_list = append(new_list, res.(float64))
			}
			return new_list, nil

		case []any:
			var seed any
			var ok bool
			switch args[2].(type) {
			case string, uint, int:
				seed, ok = GetAsFloat(args[2])
				if !ok {
					return nil, errors.New("could not convert seed")
				}
			case float64:
				seed = args[2].(float64)
			default:
				return nil, errors.New("unknown seed type")
			}
			var new_list []any
			switch args[0].(type) {
			case []float64:
				for q := range args[0].([]any) {
					gf, ok := GetAsFloat(seed)
					if !ok {
						return nil, errors.New("bad seed")
					}
					gf2, ok := GetAsFloat(args[0].([]any)[q])
					if !ok {
						return nil, errors.New("bad element")
					}
					expr := strconv.FormatFloat(gf, 'f', -1, 64) + op_string + strconv.FormatFloat(gf2, 'f', -1, 64)
					res, err := ev(reduceparser, evalfs, expr)
					if err != nil {
						return nil, errors.New("could not process list")
					}
					seed = res
					new_list = append(new_list, res)
				}
			}
			return new_list, nil
		}

		return nil, nil
	}

	slhelp["zip"] = LibHelp{in: "list1,list2", out: "list", action: "Creates a list by combining each element of [#i1]list1[#i0] and [#i1]list2[#i0]."}
	stdlib["zip"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("zip", args, 6,
			"2", "[]int", "[]int",
			"2", "[]float64", "[]float64",
			"2", "[]string", "[]string",
			"2", "[]bool", "[]bool",
			"2", "[]uint", "[]uint",
			"2", "[]interface {}", "[]interface {}"); !ok {
			return nil, err
		}

		switch args[0].(type) {
		case []bool:
			mx := max_int([]int{len(args[0].([]bool)), len(args[1].([]bool))})
			var new_list []bool
			for q := 0; q < mx; q++ {
				var a bool
				var b bool
				if q < len(args[0].([]bool)) {
					a = args[0].([]bool)[q]
				}
				if q < len(args[1].([]bool)) {
					b = args[1].([]bool)[q]
				}
				new_list = append(new_list, a, b)
			}
			return new_list, nil
		case []int:
			mx := max_int([]int{len(args[0].([]int)), len(args[1].([]int))})
			var new_list []int
			for q := 0; q < mx; q++ {
				var a int
				var b int
				if q < len(args[0].([]int)) {
					a = args[0].([]int)[q]
				}
				if q < len(args[1].([]int)) {
					b = args[1].([]int)[q]
				}
				new_list = append(new_list, a, b)
			}
			return new_list, nil
		case []uint:
			mx := max_int([]int{len(args[0].([]uint)), len(args[1].([]uint))})
			var new_list []uint
			for q := 0; q < mx; q++ {
				var a uint
				var b uint
				if q < len(args[0].([]uint)) {
					a = args[0].([]uint)[q]
				}
				if q < len(args[1].([]uint)) {
					b = args[1].([]uint)[q]
				}
				new_list = append(new_list, a, b)
			}
			return new_list, nil
		case []float64:
			mx := max_int([]int{len(args[0].([]float64)), len(args[1].([]float64))})
			var new_list []float64
			for q := 0; q < mx; q++ {
				var a float64
				var b float64
				if q < len(args[0].([]float64)) {
					a = args[0].([]float64)[q]
				}
				if q < len(args[1].([]float64)) {
					b = args[1].([]float64)[q]
				}
				new_list = append(new_list, a, b)
			}
			return new_list, nil
		case []string:
			mx := max_int([]int{len(args[0].([]string)), len(args[1].([]string))})
			var new_list []string
			for q := 0; q < mx; q++ {
				var a string
				var b string
				if q < len(args[0].([]string)) {
					a = args[0].([]string)[q]
				}
				if q < len(args[1].([]string)) {
					b = args[1].([]string)[q]
				}
				new_list = append(new_list, a, b)
			}
			return new_list, nil
		case []any:
			mx := max_int([]int{len(args[0].([]any)), len(args[1].([]any))})
			var new_list []any
			for q := 0; q < mx; q++ {
				var a any
				var b any
				if q < len(args[0].([]any)) {
					a = args[0].([]any)[q]
				}
				if q < len(args[1].([]any)) {
					b = args[1].([]any)[q]
				}
				new_list = append(new_list, a, b)
			}
			return new_list, nil
		}

		return nil, errors.New("unspecified error in zip()")
	}

	slhelp["empty"] = LibHelp{in: "list", out: "bool", action: "Is list empty?"}
	stdlib["empty"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("empty", args, 10,
			"1", "[]int",
			"1", "[]string",
			"1", "[]bool",
			"1", "[]int64",
			"1", "[]uint",
			"1", "[]float64",
			"1", "[]*big.Int",
			"1", "[]*big.Float",
			"1", "[]interface {}",
			"1", "nil"); !ok {
			return nil, err
		}

		switch args[0].(type) {
		case []string:
			if len(args[0].([]string)) == 0 {
				return true, nil
			}
		case []bool:
			if len(args[0].([]bool)) == 0 {
				return true, nil
			}
		case []int:
			if len(args[0].([]int)) == 0 {
				return true, nil
			}
		case []int64:
			if len(args[0].([]int64)) == 0 {
				return true, nil
			}
		case []uint:
			if len(args[0].([]uint)) == 0 {
				return true, nil
			}
		case []float64:
			if len(args[0].([]float64)) == 0 {
				return true, nil
			}
		case []*big.Int:
			if len(args[0].([]*big.Int)) == 0 {
				return true, nil
			}
		case []*big.Float:
			if len(args[0].([]*big.Float)) == 0 {
				return true, nil
			}
		case []any:
			if len(args[0].([]any)) == 0 {
				return true, nil
			}
		case nil:
			return true, nil
		}
		return false, nil
	}

	slhelp["col"] = LibHelp{in: "string,column[,delimiter]", out: "[]string", action: "Creates a list from a particular [#i1]column[#i0] of line separated [#i1]string[#i0]."}
	stdlib["col"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("col", args, 2,
			"3", "string", "int", "string",
			"2", "string", "int"); !ok {
			return nil, err
		}

		coln := args[1].(int)
		if coln < 1 {
			return nil, errors.New("Argument 2 (column) to col() must be a positive integer!")
		}

		var list []string
		if runtime.GOOS != "windows" {
			list = strings.Split(args[0].(string), "\n")
		} else {
			list = strings.Split(strings.Replace(args[0].(string), "\r\n", "\n", -1), "\n")
		}

		del := " "
		if len(args) == 3 {
			del = args[2].(string)
		}

		var cols []string
		if len(list) > 0 {
			for q := range list {
				z := strings.Split(list[q], del)
				if len(z) >= coln {
					cols = append(cols, z[coln-1])
				}
			}
		}
		return cols, nil
	}

	slhelp["append_to"] = LibHelp{in: "list_name,item", out: "bool_success", action: "Appends [#i1]item[#i0] to [#i1]local_list_name[#i0]. Returns [#i1]bool_success[#i0] depending on success."}
	stdlib["append_to"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("append_to", args, 1, "2", "string", "any"); !ok {
			return nil, err
		}

		if args[0] == nil {
			return nil, errors.New("first argument is not a list")
		}

		name := args[0].(string)
		bin := bind_int(evalfs, name)

		if !(*ident)[bin].declared {
			return nil, errors.New(sf("list %s does not exist", name))
		}

		// check type is compatible
		vlock.Lock()
		set := false
		switch (*ident)[bin].IValue.(type) {
		case []string:
			(*ident)[bin].IValue = append((*ident)[bin].IValue.([]string), sf("%v", args[1]))
			set = true
		case []int:
			switch args[1].(type) {
			case int:
				(*ident)[bin].IValue = append((*ident)[bin].IValue.([]int), args[1].(int))
				set = true
			}
		case []uint:
			switch args[1].(type) {
			case uint:
				(*ident)[bin].IValue = append((*ident)[bin].IValue.([]uint), args[1].(uint))
				set = true
			}
		case []float64:
			switch args[1].(type) {
			case float64:
				(*ident)[bin].IValue = append((*ident)[bin].IValue.([]float64), args[1].(float64))
				set = true
			}
		case []bool:
			switch args[1].(type) {
			case bool:
				(*ident)[bin].IValue = append((*ident)[bin].IValue.([]bool), args[1].(bool))
				set = true
			}
		case []*big.Int:
			switch args[1].(type) {
			case *big.Int:
				(*ident)[bin].IValue = append((*ident)[bin].IValue.([]*big.Int), args[1].(*big.Int))
				set = true
			}
		case []*big.Float:
			switch args[1].(type) {
			case *big.Float:
				(*ident)[bin].IValue = append((*ident)[bin].IValue.([]*big.Float), args[1].(*big.Float))
				set = true
			}
		case []any:
			(*ident)[bin].IValue = append((*ident)[bin].IValue.([]any), args[1])
			set = true
		}
		vlock.Unlock()

		if !set {
			return false, errors.New(sf("unsupported list type (%s:%T) in append_to()", args[0], (*ident)[bin].IValue))
		}

		return true, nil

	}

	// append returns a[]+arg
	slhelp["append"] = LibHelp{in: "[list,]item", out: "[]mixed", action: "Returns [#i1]new_list[#i0] containing [#i1]item[#i0] appended to [#i1]list[#i0]. If [#i1]list[#i0] is omitted then a new list is created containing [#i1]item[#i0]."}
	stdlib["append"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("append", args, 2, "1", "any", "2", "any", "any"); !ok {
			return nil, err
		}

		// should really do some kind of implicit conversion here (and elsewhere)
		// but not a high-priority, as with most things.
		if len(args) == 1 {
			switch args[0].(type) {
			case string:
				l := make([]string, 0, 31)
				return append(l, args[0].(string)), nil
			case float64:
				l := make([]float64, 0, 31)
				return append(l, args[0].(float64)), nil
			case bool:
				l := make([]bool, 0, 31)
				return append(l, args[0].(bool)), nil
			case uint:
				l := make([]uint, 0, 31)
				return append(l, args[0].(uint)), nil
			case int:
				l := make([]int, 0, 31)
				return append(l, args[0].(int)), nil
			case *big.Int:
				l := make([]*big.Int, 0, 31)
				return append(l, args[0].(*big.Int)), nil
			case *big.Float:
				l := make([]*big.Float, 0, 31)
				return append(l, args[0].(*big.Float)), nil
			case NetworkIOStats:
				l := make([]NetworkIOStats, 0, 31)
				return append(l, args[0].(NetworkIOStats)), nil
			case DiskIOStats:
				l := make([]DiskIOStats, 0, 31)
				return append(l, args[0].(DiskIOStats)), nil
			case ProcessInfo:
				l := make([]ProcessInfo, 0, 31)
				return append(l, args[0].(ProcessInfo)), nil
			case SystemResources:
				l := make([]SystemResources, 0, 31)
				return append(l, args[0].(SystemResources)), nil
			case MemoryInfo:
				l := make([]MemoryInfo, 0, 31)
				return append(l, args[0].(MemoryInfo)), nil
			case CPUInfo:
				l := make([]CPUInfo, 0, 31)
				return append(l, args[0].(CPUInfo)), nil
			case ProcessTree:
				l := make([]ProcessTree, 0, 31)
				return append(l, args[0].(ProcessTree)), nil
			case ProcessMap:
				l := make([]ProcessMap, 0, 31)
				return append(l, args[0].(ProcessMap)), nil
			case ResourceUsage:
				l := make([]ResourceUsage, 0, 31)
				return append(l, args[0].(ResourceUsage)), nil
			case ResourceSnapshot:
				l := make([]ResourceSnapshot, 0, 31)
				return append(l, args[0].(ResourceSnapshot)), nil
			case SlabInfo:
				l := make([]SlabInfo, 0, 31)
				return append(l, args[0].(SlabInfo)), nil
			case nil:
				l := make([]any, 0, 31)
				return l, nil
			case any:
				l := make([]any, 0, 31)
				return append(l, sf("%v", args[0].(any))), nil
			default:
				return nil, errors.New(sf("data type (%T) not supported in lists.", args[0]))
			}
		}

		switch args[0].(type) {
		case nil:
			switch args[1].(type) {
			case float64:
				args[0] = make([]float64, 0, 31)
			case int:
				args[0] = make([]int, 0, 31)
			case uint:
				args[0] = make([]uint, 0, 31)
			case bool:
				args[0] = make([]bool, 0, 31)
			case string:
				args[0] = make([]string, 0, 31)
			case *big.Int:
				args[0] = make([]*big.Int, 0, 31)
			case *big.Float:
				args[0] = make([]*big.Float, 0, 31)
			case any:
				args[0] = make([]any, 0, 31)
			default:
				args[0] = make([]any, 0, 31)
			}
		}

		switch s := args[0].(type) {
		case []string:
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]string, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, sf("%v", args[1]))
			return l, nil
		case []float64:
			if "float64" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:float64,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]float64, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(float64))
			return l, nil
		case []bool:
			if "bool" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:bool,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]bool, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(bool))
			return l, nil
		case []int:
			if "int" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:int,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]int, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(int))
			return l, nil
		case []*big.Int:
			if "*big.Int" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:bigi,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]*big.Int, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(*big.Int))
			return l, nil
		case []*big.Float:
			if "*big.Float" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:bigf,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]*big.Float, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(*big.Float))
			return l, nil
		case []any:
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]any, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(any))
			return l, nil
		case []NetworkIOStats:
			if "NetworkIOStats" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:NetworkIOStats,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]NetworkIOStats, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(NetworkIOStats))
			return l, nil
		case []DiskIOStats:
			if "DiskIOStats" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:DiskIOStats,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]DiskIOStats, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(DiskIOStats))
			return l, nil
		case []ProcessInfo:
			if "ProcessInfo" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:ProcessInfo,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]ProcessInfo, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(ProcessInfo))
			return l, nil
		case []SystemResources:
			if "SystemResources" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:SystemResources,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]SystemResources, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(SystemResources))
			return l, nil
		case []MemoryInfo:
			if "MemoryInfo" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:MemoryInfo,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]MemoryInfo, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(MemoryInfo))
			return l, nil
		case []CPUInfo:
			if "CPUInfo" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:CPUInfo,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]CPUInfo, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(CPUInfo))
			return l, nil
		case []ProcessTree:
			if "ProcessTree" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:ProcessTree,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]ProcessTree, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(ProcessTree))
			return l, nil
		case []ProcessMap:
			if "ProcessMap" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:ProcessMap,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]ProcessMap, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(ProcessMap))
			return l, nil
		case []ResourceUsage:
			if "ResourceUsage" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:ResourceUsage,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]ResourceUsage, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(ResourceUsage))
			return l, nil
		case []ResourceSnapshot:
			if "ResourceSnapshot" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:ResourceSnapshot,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]ResourceSnapshot, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(ResourceSnapshot))
			return l, nil
		case []SlabInfo:
			if "SlabInfo" != sf("%T", args[1]) {
				return nil, errors.New(sf("(l:SlabInfo,a:%T) data types must match in append()", args[1]))
			}
			ll := len(s)
			if ll+1 > cap(s) {
				l := make([]SlabInfo, ll, int(float64(cap(s))*appGrowthFactor))
				copy(l, s)
				s = l
			}
			l := append(s, args[1].(SlabInfo))
			return l, nil
		default:
			return nil, errors.New(sf("data type [%T] not supported in append()", args[0]))
		}
	}

	slhelp["push_front"] = LibHelp{in: "[list,]item", out: "[]mixed", action: "Adds [#i1]item[#i0] to the front of [#i1]list[#i0]. If only an item is provided, then a new list is started."}
	stdlib["push_front"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("push_front", args, 2,
			"2", "any", "any",
			"1", "any"); !ok {
			return nil, err
		}

		if len(args) == 1 {
			switch args[0].(type) {
			case string:
				l := make([]string, 0, 31)
				return append(l, args[0].(string)), nil
			case float64:
				l := make([]float64, 0, 31)
				return append(l, args[0].(float64)), nil
			case bool:
				l := make([]bool, 0, 31)
				return append(l, args[0].(bool)), nil
			case uint:
				l := make([]uint, 0, 31)
				return append(l, args[0].(uint)), nil
			case int:
				l := make([]int, 0, 31)
				return append(l, args[0].(int)), nil
			case *big.Int:
				l := make([]*big.Int, 0, 31)
				return append(l, args[0].(*big.Int)), nil
			case *big.Float:
				l := make([]*big.Float, 0, 31)
				return append(l, args[0].(*big.Float)), nil
			case any:
				l := make([]any, 0, 31)
				return append(l, sf("%v", args[0].(any))), nil
			default:
				return nil, errors.New("data type not supported in lists.")
			}
		}

		switch args[0].(type) {
		case []float64:
			if "float64" != sf("%T", args[1]) {
				return nil, errors.New("data types must match in push_front()")
			}
			l := make([]float64, 0, 31)
			l = append(l, args[1].(float64))
			l = append(l, args[0].([]float64)...)
			return l, nil
		case []bool:
			if "bool" != sf("%T", args[1]) {
				return nil, errors.New("data types must match in push_front()")
			}
			l := make([]bool, 0, 31)
			l = append(l, args[1].(bool))
			l = append(l, args[0].([]bool)...)
			return l, nil
		case []uint:
			if "uint" != sf("%T", args[1]) {
				return nil, errors.New("data types must match in push_front()")
			}
			l := make([]uint, 0, 31)
			l = append(l, args[1].(uint))
			l = append(l, args[0].([]uint)...)
			return l, nil
		case []int:
			if "int" != sf("%T", args[1]) {
				return nil, errors.New("data types must match in push_front()")
			}
			l := make([]int, 0, 31)
			l = append(l, args[1].(int))
			l = append(l, args[0].([]int)...)
			return l, nil
		case []string:
			if "string" != sf("%T", args[1]) {
				return nil, errors.New("data types must match in push_front()")
			}
			l := make([]string, 0, 31)
			l = append(l, args[1].(string))
			l = append(l, args[0].([]string)...)
			return l, nil
		case []*big.Int:
			if "*big.Int" != sf("%T", args[1]) {
				return nil, errors.New("data types must match in push_front()")
			}
			l := make([]*big.Int, 0, 31)
			l = append(l, args[1].(*big.Int))
			l = append(l, args[0].([]*big.Int)...)
			return l, nil
		case []*big.Float:
			if "*big.Float" != sf("%T", args[1]) {
				return nil, errors.New("data types must match in push_front()")
			}
			l := make([]*big.Float, 0, 31)
			l = append(l, args[1].(*big.Float))
			l = append(l, args[0].([]*big.Float)...)
			return l, nil
		case []any:
			l := make([]any, 0, 31)
			l = append(l, sf("%v", args[1].(any)))
			l = append(l, args[0].([]any)...)
			return l, nil
		default:
			return nil, errors.New("Unknown list type provided to push_front()")
		}
	}

	slhelp["peek"] = LibHelp{in: "list_name", out: "item", action: "Returns a copy of the last [#i1]item[#i0] in the list [#i1]list_name[#i0]. Returns an error if the list is empty."}
	stdlib["peek"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("peek", args, 8,
			"1", "[]string",
			"1", "[]int",
			"1", "[]uint",
			"1", "[]float64",
			"1", "[]big.Int",
			"1", "[]big.Float",
			"1", "[]bool",
			"1", "[]interface {}"); !ok {
			return nil, err
		}

		switch a := args[0].(type) {
		case []string:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		case []int:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		case []uint:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		case []float64:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		case []bool:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		case []*big.Int:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		case []*big.Float:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		case []any:
			if len(a) == 0 {
				break
			}
			return a[len(a)-1], nil
		}
		return nil, errors.New("No values available to peek()")
	}

	slhelp["pop"] = LibHelp{in: "list_name", out: "item", action: "Removes and returns the last [#i1]item[#i0] in the named list [#i1]list_name[#i0]."}
	stdlib["pop"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("pop", args, 1,
			"1", "string"); !ok {
			return nil, err
		}

		n := args[0].(string)
		v, _ := vget(nil, evalfs, ident, n)

		vlock.Lock()
		defer vlock.Unlock()

		switch v.(type) {
		case []bool:
			if ln := len(v.([]bool)); ln > 0 {
				r := v.([]bool)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]bool)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		case []int:
			if ln := len(v.([]int)); ln > 0 {
				r := v.([]int)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]int)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		case []uint:
			if ln := len(v.([]uint)); ln > 0 {
				r := v.([]uint)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]uint)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		case []float64:
			if ln := len(v.([]float64)); ln > 0 {
				r := v.([]float64)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]float64)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		case []*big.Int:
			if ln := len(v.([]*big.Int)); ln > 0 {
				r := v.([]*big.Int)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]*big.Int)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		case []*big.Float:
			if ln := len(v.([]*big.Float)); ln > 0 {
				r := v.([]*big.Float)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]*big.Float)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		case []string:
			if ln := len(v.([]string)); ln > 0 {
				r := v.([]string)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]string)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		case []any:
			if ln := len(v.([]any)); ln > 0 {
				r := v.([]any)[ln-1]
				bin := bind_int(evalfs, n)
				(*ident)[bin] = Variable{IName: n, IValue: v.([]any)[:ln-1], IKind: 0, ITyped: false, declared: true}
				return r, nil
			}
		}

		return nil, nil

	}

	slhelp["insert"] = LibHelp{in: "list,pos,item", out: "[]new_list", action: "Returns a [#i1]new_list[#i0] with [#i1]item[#i0] inserted in [#i1]list[#i0] at position [#i1]pos[#i0]. (1-based)"}
	stdlib["insert"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("insert", args, 1, "3", "any", "int", "any"); !ok {
			return nil, err
		}

		pos := args[1].(int)
		item := args[2]

		switch args[0].(type) {
		case []float64:
			l := make([]float64, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]float64)[:pos-1]...)
			}
			l = append(l, item.(float64))
			l = append(l, args[0].([]float64)[pos-1:]...)
			return l, nil
		case []string:
			l := make([]string, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]string)[:pos-1]...)
			}
			l = append(l, sf("%v", item))
			l = append(l, args[0].([]string)[pos-1:]...)
			return l, nil
		case []bool:
			l := make([]bool, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]bool)[:pos-1]...)
			}
			l = append(l, item.(bool))
			l = append(l, args[0].([]bool)[pos-1:]...)
			return l, nil
		case []int:
			l := make([]int, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]int)[:pos-1]...)
			}
			l = append(l, item.(int))
			l = append(l, args[0].([]int)[pos-1:]...)
			return l, nil
		case []uint:
			l := make([]uint, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]uint)[:pos-1]...)
			}
			l = append(l, item.(uint))
			l = append(l, args[0].([]uint)[pos-1:]...)
			return l, nil
		case []*big.Int:
			l := make([]*big.Int, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]*big.Int)[:pos-1]...)
			}
			l = append(l, item.(*big.Int))
			l = append(l, args[0].([]*big.Int)[pos-1:]...)
			return l, nil
		case []*big.Float:
			l := make([]*big.Float, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]*big.Float)[:pos-1]...)
			}
			l = append(l, item.(*big.Float))
			l = append(l, args[0].([]*big.Float)[pos-1:]...)
			return l, nil
		case []any:
			l := make([]any, 0, 31)
			if pos > 0 {
				l = append(l, args[0].([]any)[:pos-1]...)
			}
			l = append(l, item.(any))
			l = append(l, args[0].([]any)[pos-1:]...)
			return l, nil
		}
		return nil, errors.New("could not insert()")
	}

	slhelp["remove"] = LibHelp{in: "list,pos", out: "[]new_list", action: "Returns a [#i1]new_list[#i0] with the item at position [#i1]pos[#i0] removed. 1-based."}
	stdlib["remove"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("remove", args, 1, "2", "any", "int"); !ok {
			return nil, err
		}

		pos := args[1].(int)

		if pos < 1 {
			return nil, errors.New(sf("position (%v) out of range (low) in remove()", pos))
		}

		switch args[0].(type) {
		case []string:
			if pos > len(args[0].([]string)) {
				return nil, errors.New(sf("position (%v) out of range (string/high) in remove()", pos))
			}
			l := make([]string, 0, 31)
			l = append(l, args[0].([]string)[:pos-1]...)
			l = append(l, args[0].([]string)[pos:]...)
			return l, nil
		case []float64:
			if pos > len(args[0].([]float64)) {
				return nil, errors.New(sf("position (%v) out of range (float/high) in remove()", pos))
			}
			l := make([]float64, 0, 31)
			l = append(l, args[0].([]float64)[:pos-1]...)
			l = append(l, args[0].([]float64)[pos:]...)
			return l, nil
		case []bool:
			if pos > len(args[0].([]bool)) {
				return nil, errors.New(sf("position (%v) out of range (bool/high) in remove()", pos))
			}
			l := make([]bool, 0, 31)
			l = append(l, args[0].([]bool)[:pos-1]...)
			l = append(l, args[0].([]bool)[pos:]...)
			return l, nil
		case []int:
			if pos > len(args[0].([]int)) {
				return nil, errors.New(sf("position (%v) out of range (int/high) in remove()", pos))
			}
			l := make([]int, 0, 31)
			l = append(l, args[0].([]int)[:pos-1]...)
			l = append(l, args[0].([]int)[pos:]...)
			return l, nil
		case []uint:
			if pos > len(args[0].([]uint)) {
				return nil, errors.New(sf("position (%v) out of range (uint/high) in remove()", pos))
			}
			l := make([]uint, 0, 31)
			l = append(l, args[0].([]uint)[:pos-1]...)
			l = append(l, args[0].([]uint)[pos:]...)
			return l, nil
		case []*big.Int:
			if pos > len(args[0].([]*big.Int)) {
				return nil, errors.New(sf("position (%v) out of range (bigi/high) in remove()", pos))
			}
			l := make([]*big.Int, 0, 31)
			l = append(l, args[0].([]*big.Int)[:pos-1]...)
			l = append(l, args[0].([]*big.Int)[pos:]...)
			return l, nil
		case []*big.Float:
			if pos > len(args[0].([]*big.Float)) {
				return nil, errors.New(sf("position (%v) out of range (bigf/high) in remove()", pos))
			}
			l := make([]*big.Float, 0, 31)
			l = append(l, args[0].([]*big.Float)[:pos-1]...)
			l = append(l, args[0].([]*big.Float)[pos:]...)
			return l, nil
		case []any:
			if pos > len(args[0].([]any)) {
				return nil, errors.New(sf("position (%v) out of range (interface/high) in remove()", pos))
			}
			l := make([]any, 0, 31)
			l = append(l, args[0].([]any)[:pos-1]...)
			l = append(l, args[0].([]any)[pos:]...)
			return l, nil
		}
		return nil, errors.New("could not remove()")
	}

	// head(l) returns a[0]
	slhelp["head"] = LibHelp{in: "list", out: "item", action: "Returns the head element of a list."}
	stdlib["head"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("head", args, 1, "1", "any"); !ok {
			return nil, err
		}

		switch args[0].(type) {
		case []bool:
			if len(args[0].([]bool)) == 0 {
				return []bool{}, nil
			}
			return args[0].([]bool)[0], nil
		case []int:
			if len(args[0].([]int)) == 0 {
				return []int{}, nil
			}
			return args[0].([]int)[0], nil
		case []uint:
			if len(args[0].([]uint)) == 0 {
				return []uint{}, nil
			}
			return args[0].([]uint)[0], nil
		case []float64:
			if len(args[0].([]float64)) == 0 {
				return []float64{}, nil
			}
			return args[0].([]float64)[0], nil
		case []string:
			if len(args[0].([]string)) == 0 {
				return []string{}, nil
			}
			return args[0].([]string)[0], nil
		case []*big.Int:
			if len(args[0].([]*big.Int)) == 0 {
				return []*big.Int{}, nil
			}
			return args[0].([]*big.Int)[0], nil
		case []*big.Float:
			if len(args[0].([]*big.Float)) == 0 {
				return []*big.Float{}, nil
			}
			return args[0].([]*big.Float)[0], nil
		case []any:
			if len(args[0].([]any)) == 0 {
				return []any{}, nil
			}
			return args[0].([]any)[0], nil
		}
		return nil, err
	}

	// tail(l) returns a[1:]
	slhelp["tail"] = LibHelp{in: "list", out: "[]new_list", action: "Returns a new list containing all items in [#i1]list[#i0] except the head item."}
	stdlib["tail"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("tail", args, 1, "1", "any"); !ok {
			return nil, err
		}

		switch args[0].(type) {
		case []bool:
			if len(args[0].([]bool)) == 0 {
				return []bool{}, nil
			}
			return args[0].([]bool)[1:], nil
		case []int:
			if len(args[0].([]int)) == 0 {
				return []int{}, nil
			}
			return args[0].([]int)[1:], nil
		case []uint:
			if len(args[0].([]uint)) == 0 {
				return []uint{}, nil
			}
			return args[0].([]uint)[1:], nil
		case []float64:
			if len(args[0].([]float64)) == 0 {
				return []float64{}, nil
			}
			return args[0].([]float64)[1:], nil
		case []string:
			if len(args[0].([]string)) == 0 {
				return []string{}, nil
			}
			return args[0].([]string)[1:], nil
		case []*big.Int:
			if len(args[0].([]*big.Int)) == 0 {
				return []*big.Int{}, nil
			}
			return args[0].([]*big.Int)[1:], nil
		case []*big.Float:
			if len(args[0].([]*big.Float)) == 0 {
				return []*big.Float{}, nil
			}
			return args[0].([]*big.Float)[1:], nil
		case []any:
			if len(args[0].([]any)) == 0 {
				return []any{}, nil
			}
			return args[0].([]any)[1:], nil
		}
		return nil, errors.New(sf("tail() could not evaluate type %T on %#v", args[0], args[0]))
	}

	// all(l) returns bool true if a[:] all true (&&)
	slhelp["alltrue"] = LibHelp{in: "[]bool", out: "bool", action: "Returns true if all items in [#i1][]bool[#i0] evaluate to true."}
	stdlib["alltrue"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("alltrue", args, 1, "1", "[]bool"); !ok {
			return nil, err
		}
		for _, v := range args[0].([]bool) {
			if !v {
				return false, nil
			}
		}
		return true, nil
	}

	// any(l) returns bool true if a[:] any true (||)
	slhelp["anytrue"] = LibHelp{in: "[]bool", out: "boolean", action: "Returns true if any item in [#i1][]bool[#i0] evaluates to true."}
	stdlib["anytrue"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("anytrue", args, 1, "1", "[]bool"); !ok {
			return nil, err
		}
		for _, v := range args[0].([]bool) {
			if v {
				return true, nil
			}
		}
		return false, nil
	}

	// fieldsort(s,f,dir) ascending or descending sorted version returned. (type dependant)
	slhelp["fieldsort"] = LibHelp{in: "nl_string,field[,sort_type][,bool_reverse]", out: "new_string", action: "Sorts a newline separated string [#i1]nl_string[#i0] in ascending or descending ([#i1]bool_reverse[#i0]==true) order on key [#i1]field[#i0]."}
	stdlib["fieldsort"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("fieldsort", args, 3,
			"4", "string", "int", "string", "bool",
			"3", "string", "int", "string",
			"2", "string", "int"); !ok {
			return nil, err
		}

		// get list
		s := args[0].(string)

		// get column number
		var field int
		if sf("%T", args[1]) != "int" {
			return nil, errors.New("fieldsort() must be provided with a field number.")
		}
		field = args[1].(int) - 1

		// get type
		var stype string
		if len(args) > 2 {
			stype = args[2].(string)
		}

		// get direction
		var reverse bool
		if len(args) > 3 {
			reverse = args[3].(bool)
		}

		// convert string to list
		var list [][]string
		var r []string

		if runtime.GOOS != "windows" {
			r = strings.Split(s, "\n")
		} else {
			r = strings.Split(strings.Replace(s, "\r\n", "\n", -1), "\n")
		}

		for _, l := range r {
			if l == "" {
				continue
			}
			list = append(list, strings.Split(l, " "))
		}

		if field < 0 || field > len(list[0])-1 {
			return nil, errors.New(sf("Field out of range in fieldsort()\n#%v > %v\n", field, list[0]))
		}

		// build a comparison func
		var f func(int, int) bool
		switch strings.ToLower(stype) {
		case "n":
			if !reverse {
				f = func(i, j int) bool {
					ni, _ := GetAsFloat(list[i][field])
					nj, _ := GetAsFloat(list[j][field])
					return ni < nj
				}
			} else {
				f = func(i, j int) bool {
					ni, _ := GetAsFloat(list[i][field])
					nj, _ := GetAsFloat(list[j][field])
					return ni > nj
				}
			}
		case "s":
			if !reverse {
				f = func(i, j int) bool { return list[i][field] < list[j][field] }
			} else {
				f = func(i, j int) bool { return list[i][field] > list[j][field] }
			}
		case "h":
			if !reverse {
				f = func(i, j int) bool {
					return buildNum(list[i][field]) < buildNum(list[j][field])
				}
			} else {
				f = func(i, j int) bool {
					return buildNum(list[i][field]) > buildNum(list[j][field])
				}
			}
		default:
			// string sort
			if !reverse {
				f = func(i, j int) bool { return list[i][field] < list[j][field] }
			} else {
				f = func(i, j int) bool { return list[i][field] > list[j][field] }
			}
		}

		sort.SliceStable(list, f)

		// build a string
		lsep := "\n"
		if runtime.GOOS == "windows" {
			lsep = "\r\n"
		}
		var newstring strings.Builder
		newstring.Grow(100)
		for _, l := range list {
			newstring.WriteString(strings.Join(l, " ") + lsep)
		}

		return newstring.String(), nil

	}

	slhelp["ssort"] = LibHelp{in: "list,field_name[,bool_reverse]", out: "[]any", action: "Sorts a [#i1]list[#i0] of structs, on a given field name, in ascending (true) or descending (false) order."}
	stdlib["ssort"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("ssort", args, 4,
			"3", "[]any", "[]string", "[]bool",
			"3", "[]any", "string", "bool",
			"2", "[]any", "[]string",
			"2", "[]any", "string"); !ok {
			return nil, err
		}

		list := args[0]

		var field_list []string
		var direction_list []bool

		switch args[1].(type) {
		case []string:
			field_list = args[1].([]string)
			if len(args) == 3 {
				direction_list = args[2].([]bool)
			}
		case string:
			field_list = []string{args[1].(string)}
			if len(args) == 3 {
				direction_list = []bool{args[2].(bool)}
			}
		}

		outputSlice, err := MultiSorted(list, field_list, direction_list)
		if err == nil {
			ret_ar := make([]any, len(outputSlice))
			for i := range outputSlice {
				ret_ar[i] = outputSlice[i].(any)
			}
			return ret_ar, nil
		}
		return nil, err

	}

	// sort(l,[ud]) ascending or descending sorted version returned. (type dependant)
	slhelp["sort"] = LibHelp{in: "list[,bool_reverse|map_options]", out: "[]new_list", action: "Sorts a [#i1]list[#i0] in ascending or descending ([#i1]bool_reverse[#i0]==true) order, or with map options.\n" +
		"[#SOL]Map options: .reverse (bool), .numeric (bool, .alphanumeric (bool)",
	}
	stdlib["sort"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("sort", args, 5,
			"3", "any", "bool", "bool",
			"2", "any", "bool",
			"2", "any", "map",
			"1", "nil",
			"1", "any"); !ok {
			return nil, err
		}

		if args[0] == nil {
			return nil, nil
		}

		list := args[0]
		direction := false
		numeric := false
		alphanumeric := false

		if len(args) == 2 {
			switch args[1].(type) {
			case bool:
				direction = args[1].(bool)
			case map[string]any:
				options := args[1].(map[string]any)
				if reverse, ok := options["reverse"].(bool); ok {
					direction = reverse
				}
				if num, ok := options["numeric"].(bool); ok {
					numeric = num
				}
				if alpha, ok := options["alphanumeric"].(bool); ok {
					alphanumeric = alpha
				}
			}
		} else if len(args) == 3 {
			direction = args[1].(bool)
			numeric = args[2].(bool)
		}

		// need to sort?
		switch list.(type) {
		case []int:
			if len(list.([]int)) < 2 {
				return list, nil
			}
		case []uint:
			if len(list.([]uint)) < 2 {
				return list, nil
			}
		case []float64:
			if len(list.([]float64)) < 2 {
				return list, nil
			}
		case []string:
			if len(list.([]string)) < 2 {
				return list, nil
			}
		case map[string]any:
			if len(list.(map[string]any)) < 2 {
				return list, nil
			}
		case []any:
			if len(list.([]any)) < 2 {
				return list, nil
			}
		default:
			return nil, errors.New(sf("Can only sort list of type int, float or string. (This is a '%T')", list))
		}

		// sort
		switch direction {
		case false:

			switch list.(type) {

			case []int:
				sort.SliceStable(list, func(i, j int) bool { return list.([]int)[i] < list.([]int)[j] })
				return list, nil

			case []uint:
				sort.SliceStable(list, func(i, j int) bool { return list.([]uint)[i] < list.([]uint)[j] })
				return list, nil

			case []float64:
				sort.SliceStable(list, func(i, j int) bool { return list.([]float64)[i] < list.([]float64)[j] })
				return list, nil

			case []string:
				if alphanumeric {
					sort.SliceStable(list, func(i, j int) bool {
						return naturalCompare(list.([]string)[i], list.([]string)[j])
					})
				} else if numeric {
					sort.SliceStable(list, func(i, j int) bool {
						a, aErr := GetAsFloat(list.([]string)[i])
						b, bErr := GetAsFloat(list.([]string)[j])
						// If both parse as numbers, compare numerically
						if !aErr && !bErr {
							return a < b
						}
						// If only one parses as number, put numbers first
						if !aErr && bErr {
							return true
						}
						if aErr && !bErr {
							return false
						}
						// If neither parses as number, fall back to string comparison
						return list.([]string)[i] < list.([]string)[j]
					})
				} else {
					sort.SliceStable(list, func(i, j int) bool { return list.([]string)[i] < list.([]string)[j] })
				}
				return list, nil

			case []any:
				if alphanumeric {
					sort.SliceStable(list, func(i, j int) bool {
						aStr := sf("%v", list.([]any)[i])
						bStr := sf("%v", list.([]any)[j])
						return naturalCompare(aStr, bStr)
					})
				} else if numeric {
					sort.SliceStable(list, func(i, j int) bool {
						a, _ := GetAsFloat(sf("%v", list.([]any)[i]))
						b, _ := GetAsFloat(sf("%v", list.([]any)[j]))
						return a < b
					})
				} else {
					sort.SliceStable(list, func(i, j int) bool { return sf("%v", list.([]any)[i]) < sf("%v", list.([]any)[j]) })
				}
				return list, nil

			// @note: ignore this, placeholder until we can do something useful here...
			case map[string]any:

				var iter *reflect.MapIter
				iter = reflect.ValueOf(list.(map[string]any)).MapRange()
				iter.Next()
				switch iter.Value().Interface().(type) {
				case int:
					kv := make([]sortStructInt, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructInt{k: k, v: v.(int)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v < kv[j].v })
					l := make(map[string]int)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return l, nil
				case uint:
					kv := make([]sortStructUint, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructUint{k: k, v: v.(uint)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v < kv[j].v })
					l := make(map[string]uint)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return l, nil
				case float64:
					kv := make([]sortStructFloat, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructFloat{k: k, v: v.(float64)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v < kv[j].v })
					l := make(map[string]float64)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return l, nil
				case string:
					kv := make([]sortStructString, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructString{k: k, v: v.(string)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v < kv[j].v })
					l := make(map[string]string)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return l, nil
				case any:
					kv := make([]sortStructInterface, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructInterface{k: k, v: v})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].k < kv[j].k })
					l := make(map[string]any)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return l, nil
				default:
					pf("Error: unknown type '%T' in sort()\n", list)
					finish(false, ERR_EVAL)
				}
				return args[0], nil
			}

		case true: // descending

			switch list.(type) {

			case []int:
				sort.SliceStable(list, func(i, j int) bool { return list.([]int)[i] > list.([]int)[j] })
				return list, nil

			case []uint:
				sort.SliceStable(list, func(i, j int) bool { return list.([]uint)[i] > list.([]uint)[j] })
				return list, nil

			case []float64:
				sort.SliceStable(list, func(i, j int) bool { return list.([]float64)[i] > list.([]float64)[j] })
				return list, nil

			case []string:
				if alphanumeric {
					sort.SliceStable(list, func(i, j int) bool {
						return naturalCompare(list.([]string)[i], list.([]string)[j])
					})
				} else if numeric {
					sort.SliceStable(list, func(i, j int) bool {
						a, aErr := GetAsFloat(list.([]string)[i])
						b, bErr := GetAsFloat(list.([]string)[j])
						// If both parse as numbers, compare numerically
						if !aErr && !bErr {
							return a > b
						}
						// If only one parses as number, put numbers first
						if !aErr && bErr {
							return true
						}
						if aErr && !bErr {
							return false
						}
						// If neither parses as number, fall back to string comparison
						return list.([]string)[i] > list.([]string)[j]
					})
				} else {
					sort.SliceStable(list, func(i, j int) bool { return list.([]string)[i] > list.([]string)[j] })
				}
				return list, nil

			case []any:
				if alphanumeric {
					sort.SliceStable(list, func(i, j int) bool {
						aStr := sf("%v", list.([]any)[i])
						bStr := sf("%v", list.([]any)[j])
						return naturalCompare(aStr, bStr)
					})
				} else if numeric {
					sort.SliceStable(list, func(i, j int) bool {
						a, _ := GetAsFloat(sf("%v", list.([]any)[i]))
						b, _ := GetAsFloat(sf("%v", list.([]any)[j]))
						return a > b
					})
				} else {
					sort.SliceStable(list, func(i, j int) bool { return sf("%v", list.([]any)[i]) > sf("%v", list.([]any)[j]) })
				}
				return list, nil

			// placeholders again...
			case map[string]any:
				var iter *reflect.MapIter
				iter = reflect.ValueOf(list.(map[string]any)).MapRange()
				iter.Next()
				switch iter.Value().Interface().(type) {
				case int:
					kv := make([]sortStructInt, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructInt{k: k, v: v.(int)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v > kv[j].v })
					l := make(map[string]int)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return kv, nil
				case uint:
					kv := make([]sortStructUint, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructUint{k: k, v: v.(uint)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v > kv[j].v })
					l := make(map[string]uint)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return kv, nil
				case float64:
					kv := make([]sortStructFloat, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructFloat{k: k, v: v.(float64)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v > kv[j].v })
					l := make(map[string]float64)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return kv, nil
				case string:
					kv := make([]sortStructString, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructString{k: k, v: v.(string)})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].v > kv[j].v })
					l := make(map[string]string)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return l, nil
				case any:
					kv := make([]sortStructInterface, 0, len(list.(map[string]any)))
					for k, v := range list.(map[string]any) {
						kv = append(kv, sortStructInterface{k: k, v: v})
					}
					sort.Slice(kv, func(i, j int) bool { return kv[i].k > kv[j].k })
					l := make(map[string]any)
					for _, v := range kv {
						l[v.k] = v.v
					}
					return l, nil
				default:
					pf("Error: unknown type '%T' in sort()\n", list)
					finish(false, ERR_EVAL)
				}
				return args[0], nil
			}

		}
		return args[0], nil
	}

	slhelp["list_bool"] = LibHelp{in: "int_or_string_list", out: "[]bool", action: "Returns [#i1]int_or_string_list[#i0] as a list of boolean values, with invalid items removed."}
	stdlib["list_bool"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("list_bool", args, 5,
			"1", "[]int",
			"1", "[]uint",
			"1", "[]float64",
			"1", "[]string",
			"1", "[]interface {}"); !ok {
			return nil, err
		}

		var bool_list []bool
		switch args[0].(type) {
		case []int:
			for _, q := range args[0].([]int) {
				bool_list = append(bool_list, q != 0)
			}
		case []uint:
			for _, q := range args[0].([]uint) {
				bool_list = append(bool_list, q != 0)
			}
		case []float64:
			for _, q := range args[0].([]float64) {
				bool_list = append(bool_list, q != 0 && !math.IsNaN(q))
			}
		case []string:
			for _, q := range args[0].([]string) {
				switch strings.ToLower(q) {
				case "true":
					bool_list = append(bool_list, true)
				case "false":
					bool_list = append(bool_list, false)
				default:
					v, invalid := GetAsInt(sf("%v", q))
					if !invalid {
						bool_list = append(bool_list, v != 0)
					}
				}
			}
		case []any:
			for _, q := range args[0].([]any) {
				v := sf("%v", q)
				switch strings.ToLower(v) {
				case "true":
					bool_list = append(bool_list, true)
				case "false":
					bool_list = append(bool_list, false)
				default:
					v2, invalid := GetAsInt(sf("%v", q))
					if !invalid {
						bool_list = append(bool_list, v2 != 0)
					}
				}
			}
		}
		return bool_list, nil
	}

	slhelp["list_float"] = LibHelp{in: "int_or_string_list", out: "[]float_list", action: "Returns [#i1]int_or_string_list[#i0] as a list of floats, with invalid items removed."}
	stdlib["list_float"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("list_float", args, 5,
			"1", "[]int",
			"1", "[]uint",
			"1", "[]float64",
			"1", "[]string",
			"1", "[]interface {}"); !ok {
			return nil, err
		}

		var float_list []float64
		switch args[0].(type) {
		case []float64:
			return args[0].([]float64), nil
		case []int:
			for _, q := range args[0].([]int) {
				v, invalid := GetAsFloat(sf("%v", q))
				if !invalid {
					float_list = append(float_list, v)
				}
			}
		case []uint:
			for _, q := range args[0].([]uint) {
				v, invalid := GetAsFloat(sf("%v", q))
				if !invalid {
					float_list = append(float_list, v)
				}
			}
		case []string:
			for _, q := range args[0].([]string) {
				v, invalid := GetAsFloat(sf("%v", q))
				if !invalid {
					float_list = append(float_list, v)
				}
			}
		case []any:
			for _, q := range args[0].([]any) {
				v, invalid := GetAsFloat(sf("%v", q))
				if !invalid {
					float_list = append(float_list, v)
				}
			}
		}
		return float_list, nil
	}

	slhelp["list_bigi"] = LibHelp{in: "list", out: "[]bigi_list", action: "Returns [#i1]list[#i0] as a list of big integer containers. Invalid elements are discarded."}
	stdlib["list_bigi"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("list_bigi", args, 1,
			"1", "[]interface {}"); !ok {
			return nil, err
		}

		var int_list []*big.Int
		switch args[0].(type) {
		case []any:
			for _, q := range args[0].([]any) {
				v := GetAsBigInt(q)
				int_list = append(int_list, v)
			}
		}
		return int_list, nil
	}

	slhelp["list_bigf"] = LibHelp{in: "list", out: "[]bigf_list", action: "Returns [#i1]list[#i0] as a list of big float containers. Invalid elements are discarded."}
	stdlib["list_bigf"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("list_bigf", args, 1,
			"1", "[]interface {}"); !ok {
			return nil, err
		}

		var float_list []*big.Float
		switch args[0].(type) {
		case []any:
			for _, q := range args[0].([]any) {
				v := GetAsBigFloat(q)
				float_list = append(float_list, v)
			}
		}
		return float_list, nil
	}

	slhelp["list_int"] = LibHelp{in: "float_or_string_list", out: "[]int_list", action: "Returns [#i1]float_or_string_list[#i0] as a list of integers. Invalid items will generate an error."}
	stdlib["list_int"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("list_int", args, 7,
			"1", "[]int",
			"1", "[]uint",
			"1", "[]int64",
			"1", "[]bool",
			"1", "[]float64",
			"1", "[]string",
			"1", "[]interface {}"); !ok {
			return nil, err
		}

		var int_list []int
		switch args[0].(type) {
		case []int:
			return args[0].([]int), nil
		case []int64:
			return args[0].([]int64), nil
		case []uint:
			for _, q := range args[0].([]uint) {
				v, invalid := GetAsInt(q)
				if !invalid {
					int_list = append(int_list, v)
				} else {
					return nil, errors.New(sf("could not treat %v as an integer.", q))
				}
			}
		case []bool:
			var tv int
			for _, q := range args[0].([]bool) {
				if q {
					tv = 1
				} else {
					tv = 0
				}
				int_list = append(int_list, tv)
			}
		case []float64:
			for _, q := range args[0].([]float64) {
				v, invalid := GetAsInt(q)
				if !invalid {
					int_list = append(int_list, v)
				} else {
					return nil, errors.New(sf("could not treat %v as an integer.", q))
				}
			}
		case []string:
			for _, q := range args[0].([]string) {
				v, invalid := GetAsInt(sf("%v", q))
				if !invalid {
					int_list = append(int_list, v)
				} else {
					return nil, errors.New(sf("could not treat %v as an integer.", q))
				}
			}
		case []any:
			for _, q := range args[0].([]any) {
				v, invalid := GetAsInt(sf("%v", q))
				if !invalid {
					int_list = append(int_list, v)
				} else {
					return nil, errors.New(sf("could not treat %v as an integer.", q))
				}
			}
		}
		return int_list, nil
	}

	slhelp["list_int64"] = LibHelp{in: "list", out: "[]int64_list", action: "Returns [#i1]list[#i0] as a list of 64-bit integers."}
	stdlib["list_int64"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("list_int64", args, 7,
			"1", "[]int",
			"1", "[]uint",
			"1", "[]int64",
			"1", "[]float64",
			"1", "[]string",
			"1", "[]bool",
			"1", "[]interface {}"); !ok {
			return nil, err
		}

		var int64_list []int64
		switch args[0].(type) {
		case []int64:
			return args[0].([]int64), nil
		case []int:
			for _, q := range args[0].([]int) {
				int64_list = append(int64_list, int64(q))
			}
		case []uint:
			for _, q := range args[0].([]uint) {
				int64_list = append(int64_list, int64(q))
			}
		case []float64:
			for _, q := range args[0].([]float64) {
				int64_list = append(int64_list, int64(q))
			}
		case []string:
			for _, q := range args[0].([]string) {
				if v, err := strconv.ParseInt(q, 10, 64); err == nil {
					int64_list = append(int64_list, v)
				} else {
					return nil, errors.New(sf("could not treat '%s' as int64.", q))
				}
			}
		case []bool:
			for _, q := range args[0].([]bool) {
				if q {
					int64_list = append(int64_list, 1)
				} else {
					int64_list = append(int64_list, 0)
				}
			}
		case []any:
			for _, q := range args[0].([]any) {
				switch v := q.(type) {
				case int64:
					int64_list = append(int64_list, v)
				case int:
					int64_list = append(int64_list, int64(v))
				case uint:
					int64_list = append(int64_list, int64(v))
				case float64:
					int64_list = append(int64_list, int64(v))
				case string:
					if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
						int64_list = append(int64_list, parsed)
					} else {
						return nil, errors.New(sf("could not treat '%s' as int64.", v))
					}
				case bool:
					if v {
						int64_list = append(int64_list, 1)
					} else {
						int64_list = append(int64_list, 0)
					}
				default:
					return nil, errors.New(sf("could not treat %v as int64.", q))
				}
			}
		}
		return int64_list, nil
	}

	slhelp["list_string"] = LibHelp{in: "list", out: "[]string_list", action: "Converts [#i1]list[#i0] to a list of strings."}
	stdlib["list_string"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("list_int", args, 7,
			"1", "[]int",
			"1", "[]uint",
			"1", "[]int64",
			"1", "[]float64",
			"1", "[]string",
			"1", "[]bool",
			"1", "[]interface {}"); !ok {
			return nil, err
		}
		var string_list []string
		switch args[0].(type) {
		case []string:
			return args[0].([]string), nil
		case []float64:
			for _, q := range args[0].([]float64) {
				string_list = append(string_list, strconv.FormatFloat(q, 'f', -1, 64))
			}
		case []int:
			for _, q := range args[0].([]int) {
				string_list = append(string_list, strconv.FormatInt(int64(q), 10))
			}
		case []int64:
			for _, q := range args[0].([]int64) {
				string_list = append(string_list, strconv.FormatInt(q, 10))
			}
		case []uint:
			for _, q := range args[0].([]uint) {
				string_list = append(string_list, strconv.FormatUint(uint64(q), 10))
			}
		case []bool:
			for _, q := range args[0].([]bool) {
				string_list = append(string_list, strconv.FormatBool(q))
			}
		case []any:
			for _, q := range args[0].([]any) {
				string_list = append(string_list, sf("%v", q))
			}
		}
		return string_list, nil
	}

	// uniq(l) returns a sorted list with duplicates removed
	slhelp["uniq"] = LibHelp{in: "[]list", out: "[]new_list", action: "Returns [#i1]list[#i0] sorted with duplicate values removed."}
	stdlib["uniq"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("uniq", args, 5,
			"1", "string",
			"1", "[]string",
			"1", "[]int",
			"1", "[]float64",
			"1", "[]uint"); !ok {
			return nil, err
		}

		switch args[0].(type) {
		case string:
			var ns strings.Builder
			ns.Grow(100)

			var first bool = true
			var prev string

			lsep := "\n"
			var r []string
			if runtime.GOOS != "windows" {
				r = strings.Split(args[0].(string), "\n")
			} else {
				r = strings.Split(strings.Replace(args[0].(string), "\r\n", "\n", -1), "\n")
				lsep = "\r\n"
			}

			for _, v := range r {
				if first {
					first = false
					ns.WriteString(v + lsep)
					prev = v
					continue
				}
				if v == prev {
					continue
				}
				ns.WriteString(v + lsep)
				prev = v
			}

			return ns.String(), nil

		case []float64:
			var newlist []float64
			sort.SliceStable(args[0].([]float64), func(i, j int) bool { return args[0].([]float64)[i] < args[0].([]float64)[j] })
			if len(args[0].([]float64)) > 1 {
				newlist = append(newlist, args[0].([]float64)[0])
				for p := 1; p < len(args[0].([]float64)); p++ {
					prev := args[0].([]float64)[p-1]
					if args[0].([]float64)[p] == prev {
						continue
					}
					newlist = append(newlist, args[0].([]float64)[p])
				}
				return newlist, nil
			} else {
				return args[0].([]float64), nil
			}

		case []int:
			var newlist []int
			sort.SliceStable(args[0].([]int), func(i, j int) bool { return args[0].([]int)[i] < args[0].([]int)[j] })
			if len(args[0].([]int)) > 1 {
				newlist = append(newlist, args[0].([]int)[0])
				for p := 1; p < len(args[0].([]int)); p++ {
					prev := args[0].([]int)[p-1]
					if args[0].([]int)[p] == prev {
						continue
					}
					newlist = append(newlist, args[0].([]int)[p])
				}
				return newlist, nil
			} else {
				return args[0].([]int), nil
			}

		case []uint:
			var newlist []uint
			sort.SliceStable(args[0].([]uint), func(i, j int) bool { return args[0].([]uint)[i] < args[0].([]uint)[j] })
			if len(args[0].([]uint)) > 1 {
				newlist = append(newlist, args[0].([]uint)[0])
				for p := 1; p < len(args[0].([]uint)); p++ {
					prev := args[0].([]uint)[p-1]
					if args[0].([]uint)[p] == prev {
						continue
					}
					newlist = append(newlist, args[0].([]uint)[p])
				}
				return newlist, nil
			} else {
				return args[0].([]uint), nil
			}

		case []string:
			var newlist []string
			sort.SliceStable(args[0].([]string), func(i, j int) bool { return args[0].([]string)[i] < args[0].([]string)[j] })
			if len(args[0].([]string)) > 1 {
				newlist = append(newlist, args[0].([]string)[0])
				for p := 1; p < len(args[0].([]string)); p++ {
					prev := args[0].([]string)[p-1]
					if args[0].([]string)[p] == prev {
						continue
					}
					newlist = append(newlist, args[0].([]string)[p])
				}
				return newlist, nil
			} else {
				return args[0].([]string), nil
			}

		default:
			return args[0].([]any), errors.New("uniq() can only operate upon lists of type float, int or string.")
		}
	}

	// concat(l1,l2) returns concatenated list of l1,l2
	slhelp["concat"] = LibHelp{in: "list,list", out: "[]new_list", action: "(deprecated) Concatenates two lists and returns the result."}
	stdlib["concat"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("concat", args, 1, "2", "any", "any"); !ok {
			return nil, err
		}

		if reflect.TypeOf(args[0]) != reflect.TypeOf(args[1]) {
			return nil, errors.New("Cannot concatenate dissimilar type lists.")
		}

		switch args[0].(type) {
		case []bool:
			return append(args[0].([]bool), args[1].([]bool)...), nil
		case []int:
			return append(args[0].([]int), args[1].([]int)...), nil
		case []uint:
			return append(args[0].([]uint), args[1].([]uint)...), nil
		case []string:
			return append(args[0].([]string), args[1].([]string)...), nil
		case []float64:
			return append(args[0].([]float64), args[1].([]float64)...), nil
		case []any:
			return append(args[0].([]any), args[1].([]any)...), nil
		}
		return nil, errors.New(sf("Unknown list type concatenation (%T+%T)", args[0], args[1]))
	}

	// esplit(l,"a","b",match) recreates l with a[:match] and returns success flag
	slhelp["esplit"] = LibHelp{in: `[]list,"var1","var2",pos`, out: "bool", action: "Split [#i1]list[#i0] at position [#i1]pos[#i0] (1-based). Each side is put into variables [#i1]var1[#i0] and [#i1]var2[#i0]."}
	stdlib["esplit"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("esplit", args, 1, "4", "any", "string", "string", "int"); !ok {
			return nil, err
		}

		// pf("in esplit : arg 1 : %s\n",args[1].(string))
		// pf("in esplit : arg 2 : %s\n",args[2].(string))

		switch args[0].(type) {
		case []bool, []string, []uint8, []int, []uint, []float64, []any:
		default:
			return false, errors.New("Argument 1 must be a list.")
		}
		pos := args[3].(int)

		invalidPos := false
		switch args[0].(type) {
		case []float64:
			if pos < 0 || pos > len(args[0].([]float64)) {
				invalidPos = true
				break
			}
			vset(nil, evalfs, ident, args[1].(string), args[0].([]float64)[:pos-1])
			vset(nil, evalfs, ident, args[2].(string), args[0].([]float64)[pos-1:])
		case []bool:
			if pos < 0 || pos > len(args[0].([]bool)) {
				invalidPos = true
				break
			}
			vset(nil, evalfs, ident, args[1].(string), args[0].([]bool)[:pos-1])
			vset(nil, evalfs, ident, args[2].(string), args[0].([]bool)[pos-1:])
		case []int:
			if pos < 0 || pos > len(args[0].([]int)) {
				invalidPos = true
				break
			}
			vset(nil, evalfs, ident, args[1].(string), args[0].([]int)[:pos-1])
			vset(nil, evalfs, ident, args[2].(string), args[0].([]int)[pos-1:])
		case []uint:
			if pos < 0 || pos > len(args[0].([]uint)) {
				invalidPos = true
				break
			}
			vset(nil, evalfs, ident, args[1].(string), args[0].([]uint)[:pos-1])
			vset(nil, evalfs, ident, args[2].(string), args[0].([]uint)[pos-1:])
		case []string:
			if pos < 0 || pos > len(args[0].([]string)) {
				invalidPos = true
				break
			}
			vset(nil, evalfs, ident, args[1].(string), args[0].([]string)[:pos-1])
			vset(nil, evalfs, ident, args[2].(string), args[0].([]string)[pos-1:])
		case []any:
			if pos < 0 || pos > len(args[0].([]any)) {
				invalidPos = true
				break
			}
			vset(nil, evalfs, ident, args[1].(string), args[0].([]any)[:pos-1])
			vset(nil, evalfs, ident, args[2].(string), args[0].([]any)[pos-1:])
		}
		if invalidPos {
			return false, errors.New("List position not within a valid range.")
		}
		return true, nil
	}

	/*
	   // @note: this one is deliberately removed. it has issues.
	   // msplit(l,match) recreates l with a[:matching_element_pos_of(match)] and returns status
	   slhelp["msplit"] = LibHelp{in: `[]list,"var1","var2",match`, out: "bool", action: "Split [#i1]list[#i0] at first item matching [#i1]match[#i0]. Each side is put into variables [#i1]var1[#i0] and [#i1]var2[#i0]. Returns success flag."}
	   stdlib["msplit"] = func(ns string,evalfs uint32,ident *[]Variable,args ...any) (ret any, err error) {
	       if ok,err:=expect_args("msplit",args,2,
	           "4","[]string","string","string","string",
	           "4","[]interface {}","string","string","string"); !ok { return nil,err }

	       var pos int = -1
	       switch args[0].(type) {
	       case []string:
	           for q, v := range args[0].([]string) {
	               if match, _ := regexp.MatchString(args[3].(string), v); match {
	                   pos = q
	                   break
	               }
	           }
	       case []any:
	           for q, v := range args[0].([]any) {
	               if match, _ := regexp.MatchString(args[3].(string), v.(string)); match {
	                   pos = q
	                   break
	               }
	           }
	       }

	       if pos == -1 {
	           return false, nil
	       }

	       switch args[0].(type) {
	       case []string:
	           if pos < 0 || pos > len(args[0].([]string)) {
	               return false, errors.New("List position not within a valid range.")
	           }
	           vset(nil,evalfs,ident, args[1].(string), args[0].([]string)[:pos])
	           vset(nil,evalfs,ident, args[2].(string), args[0].([]string)[pos:])
	       case []any:
	           if pos < 0 || pos > len(args[0].([]any)) {
	               return false, errors.New("List position not within a valid range.")
	           }
	           vset(nil,evalfs,ident, args[1].(string), args[0].([]any)[:pos])
	           vset(nil,evalfs,ident, args[2].(string), args[0].([]any)[pos:])
	       }
	       return true, nil

	   }
	*/

	slhelp["eqlen"] = LibHelp{in: "list_of_lists_or_strings", out: "bool", action: "Checks that all lists or strings contained in the input are of equal length."}
	stdlib["eqlen"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("eqlen", args, 1, "1", "[]interface {}"); !ok {
			return nil, err
		}
		switch args[0].(type) {
		case []any:
			var ll int
			for k, l := range args[0].([]any) {
				switch l := l.(type) {
				case []int:
					if k != 0 && len(l) != ll {
						return false, nil
					}
					ll = len(l)
				case []uint:
					if k != 0 && len(l) != ll {
						return false, nil
					}
					ll = len(l)
				case []float64:
					if k != 0 && len(l) != ll {
						return false, nil
					}
					ll = len(l)
				case []bool:
					if k != 0 && len(l) != ll {
						return false, nil
					}
					ll = len(l)
				case string:
					if k != 0 && len(l) != ll {
						return false, nil
					}
					ll = len(l)
				case []any:
					if k != 0 && len(l) != ll {
						return false, nil
					}
					ll = len(l)
				default:
					return false, errors.New(sf("Not a valid type [%T] in eqlen()", l))
				}
			}
			return true, nil
		}
		return false, errors.New(sf("Not a valid list of lists or strings [%T] in eqlen()", args[0]))
	}

	slhelp["list_fill"] = LibHelp{in: "list,value[,start,end]", out: "list", action: "sets all elements of a [#i1]list[#i0] to [#i1]value[#i0]. Big number types not yet supported."}
	stdlib["list_fill"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {

		if ok, err := expect_args("list_fill", args, 12,
			"2", "[]any", "any",
			"2", "[]int", "int",
			"2", "[]uint", "uint",
			"2", "[]float64", "float64",
			"2", "[]string", "string",
			"2", "[]bool", "bool",
			"4", "[]any", "any", "int", "int",
			"4", "[]int", "int", "int", "int",
			"4", "[]uint", "uint", "int", "int",
			"4", "[]float64", "float64", "int", "int",
			"4", "[]string", "string", "int", "int",
			"4", "[]bool", "bool", "int", "int"); !ok {
			return nil, err
		}

		pos_start := 0
		pos_end := -1
		if len(args) == 4 {
			pos_start = args[2].(int)
			pos_end = args[3].(int)
		}

		switch args[0].(type) {
		case []any:
			ll := len(args[0].([]any))
			if pos_end == -1 {
				pos_end = ll - 1
			}
			l := make([]any, ll)
			copy(l, args[0].([]any))
			for i := pos_start; i <= pos_end; i += 1 {
				l[i] = args[1]
			}
			return l, nil
		case []int:
			ll := len(args[0].([]int))
			if pos_end == -1 {
				pos_end = ll - 1
			}
			l := make([]int, ll)
			copy(l, args[0].([]int))
			for i := pos_start; i <= pos_end; i += 1 {
				l[i] = args[1].(int)
			}
			return l, nil
		case []uint:
			ll := len(args[0].([]uint))
			if pos_end == -1 {
				pos_end = ll - 1
			}
			l := make([]uint, ll)
			copy(l, args[0].([]uint))
			for i := pos_start; i <= pos_end; i += 1 {
				l[i] = args[1].(uint)
			}
			return l, nil
		case []bool:
			ll := len(args[0].([]bool))
			if pos_end == -1 {
				pos_end = ll - 1
			}
			l := make([]bool, ll)
			copy(l, args[0].([]bool))
			for i := pos_start; i <= pos_end; i += 1 {
				l[i] = args[1].(bool)
			}
			return l, nil
		case []string:
			ll := len(args[0].([]string))
			if pos_end == -1 {
				pos_end = ll - 1
			}
			l := make([]string, ll)
			copy(l, args[0].([]string))
			for i := pos_start; i <= pos_end; i += 1 {
				l[i] = args[1].(string)
			}
			return l, nil
		case []float64:
			ll := len(args[0].([]float64))
			if pos_end == -1 {
				pos_end = ll - 1
			}
			l := make([]float64, ll)
			copy(l, args[0].([]float64))
			for i := pos_start; i <= pos_end; i += 1 {
				l[i] = args[1].(float64)
			}
			return l, nil
		}
		return nil, errors.New(sf("Invalid return from fill()"))
	}

	slhelp["min"] = LibHelp{in: "list", out: "number", action: "Calculate the minimum value in a [#i1]list[#i0]. Supports multi-dimensional arrays."}
	stdlib["min"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("min", args, 1, "1", "any"); !ok {
			return nil, err
		}

		// Check if it's a slice (1D or multi-dimensional)
		if isSlice(args[0]) {
			return min_multi(args[0]), nil
		} else {
			// Handle scalar case
			f, hasError := GetAsFloat(args[0])
			if hasError {
				return nil, errors.New(sf("Cannot convert scalar to number in min(): %v", args[0]))
			}
			return f, nil
		}
	}

	slhelp["max"] = LibHelp{in: "list", out: "number", action: "Calculate the maximum value in a [#i1]list[#i0]. Supports multi-dimensional arrays."}
	stdlib["max"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("max", args, 1, "1", "any"); !ok {
			return nil, err
		}

		// Check if it's a slice (1D or multi-dimensional)
		if isSlice(args[0]) {
			return max_multi(args[0]), nil
		} else {
			// Handle scalar case
			f, hasError := GetAsFloat(args[0])
			if hasError {
				return nil, errors.New(sf("Cannot convert scalar to number in max(): %v", args[0]))
			}
			return f, nil
		}
	}

	slhelp["avg"] = LibHelp{in: "list", out: "number", action: "Calculate the average value in a [#i1]list[#i0]. Supports multi-dimensional arrays."}
	stdlib["avg"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("avg", args, 1, "1", "any"); !ok {
			return nil, err
		}

		// Check if it's a slice (1D or multi-dimensional)
		if isSlice(args[0]) {
			return avg_multi(args[0]), nil
		} else {
			// Handle scalar case
			f, hasError := GetAsFloat(args[0])
			if hasError {
				return nil, errors.New(sf("Cannot convert scalar to number in avg(): %v", args[0]))
			}
			return f, nil
		}
	}

	slhelp["sum"] = LibHelp{in: "list,axis,keepdims", out: "number|array", action: "Calculate the sum of values. axis: -1/None=flatten, 0=first dim, 1=second dim. keepdims: preserve dimensions."}
	stdlib["sum"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if len(args) == 0 || len(args) > 3 {
			return nil, errors.New("sum: requires 1-3 arguments (list, axis?, keepdims?)")
		}

		list := args[0]
		if !isSlice(list) {
			return nil, errors.New("sum: first argument must be a list")
		}

		// Default behavior (backward compatibility)
		if len(args) == 1 {
			return sum_multi(list), nil
		}

		// Process axis and keepdims parameters
		axis, keepdims, err := processAxisParametersList(args[1:])
		if err != nil {
			return nil, err
		}

		// Apply operation along axis
		result, err := applyAlongAxisList(list, axis, func(slice []any) any {
			return sum_multi(slice)
		})
		if err != nil {
			return nil, err
		}

		// Apply keepdims if requested
		if keepdims {
			dims := getArrayDimensionsList(list)
			result = applyKeepdimsList(result, dims, axis, true)
		}

		return result, nil
	}

}
