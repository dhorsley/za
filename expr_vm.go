package main

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

// ExprVM is the stack-based virtual machine for executing bytecode.
type ExprVM struct {
	stack        []any
	sp           int
	fs           uint32
	ident        *[]Variable
	midentFS     uint32
	pool         []any
	namespace    string
	withEnumName string
}

// vmPool reuses VM instances to avoid allocation overhead in hot loops.
var vmPool = sync.Pool{
	New: func() any {
		return &ExprVM{
			stack: make([]any, 1024),
		}
	},
}

func runExprVM(code []Instr, pool []any, fs uint32, ident *[]Variable, midentFS uint32, withEnumName string, sourceLine int) (result any, retErr error) {
	vm := vmPool.Get().(*ExprVM)
	vm.sp = 0
	vm.fs = fs
	vm.ident = ident
	vm.midentFS = midentFS
	vm.pool = pool
	vm.namespace = "main"
	vm.withEnumName = withEnumName

	defer func() {
		if r := recover(); r != nil {
			vmPool.Put(vm)
			// Convert panic to exception (like dparse() does)
			var errVal error
			switch v := r.(type) {
			case error:
				errVal = v
			default:
				errVal = fmt.Errorf("%v", r)
			}
			var stackTraceCopy []stackFrame
			if fs < uint32(len(calltable)) {
				stackTraceCopy = generateStackTrace(calltable[fs].fs, fs, int16(sourceLine))
			}
			// Use try block's default category if set (e.g. try throws "database")
			var category any = "error"
			if fs < uint32(len(calltable)) {
				calllock.RLock()
				defaultCategory := calltable[fs].defaultExceptionCategory
				calllock.RUnlock()
				if defaultCategory != nil {
					category = defaultCategory
				}
			}
			excInfo := &exceptionInfo{
				category:   category,
				message:    errVal.Error(),
				line:       sourceLine,
				function:   calltable[fs].fs,
				fs:         fs,
				stackTrace: stackTraceCopy,
			}
			atomic.StorePointer(&calltable[fs].activeException, unsafe.Pointer(excInfo))
			// Don't set retErr — let the statement loop detect activeException
			// and route to catch blocks (matching dparse's behaviour at eval.go:206)
		}
	}()

	for pc := 0; pc < len(code); pc++ {
		instr := code[pc]

		switch instr.Op {
		case OpLoadConstInt:
			vm.push(pool[instr.Arg1])
		case OpLoadConstFloat:
			vm.push(pool[instr.Arg1])
		case OpLoadConstString:
			vm.push(pool[instr.Arg1])
		case OpLoadConstBool:
			vm.push(instr.Arg1 != 0)
		case OpLoadConstSmallInt:
			vm.push(int(int16(instr.Arg1)))
		case OpLoadNil:
			vm.push(nil)
		case OpLoadLocal:
			bin := uint64(instr.Arg1)
			if bin < uint64(len(*vm.ident)) && (*vm.ident)[bin].declared {
				vm.push((*vm.ident)[bin].IValue)
			} else {
				vmPool.Put(vm)
				return nil, fmt.Errorf("uninitialised local")
			}
		case OpLoadGlobal:
			bin := uint64(instr.Arg1)
			if bin < uint64(len(mident)) && mident[bin].declared {
				vm.push(mident[bin].IValue)
			} else {
				vmPool.Put(vm)
				return nil, fmt.Errorf("uninitialised global")
			}
		case OpLoadIdent:
			name := vm.pool[instr.Arg1].(string)
			if instr.Arg2 != 0 && vm.ident != nil {
				bin := uint64(instr.Arg2 - 1)
				if bin < uint64(len(*vm.ident)) && (*vm.ident)[bin].declared && (*vm.ident)[bin].IName == name {
					vm.push((*vm.ident)[bin].IValue)
					continue
				}
			}
			val, ok := vm.resolveIdent(name)
			if !ok {
				vmPool.Put(vm)
				return nil, fmt.Errorf("'%s' is uninitialised", name)
			}
			vm.push(val)
		case OpStoreLocal:
			val := vm.pop()
			bin := uint64(instr.Arg1)
			name := vm.pool[instr.Arg2].(string)
			vm.storeLocal(bin, name, val)
		case OpPop:
			vm.pop()
		case OpDup:
			vm.push(vm.peek())
		case OpAddInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a + b)
		case OpAddFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a + b)
		case OpAddString:
			b := vm.pop().(string)
			a := vm.pop().(string)
			vm.push(a + b)
		case OpAddGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_add(a, b))
		case OpSubInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a - b)
		case OpSubFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a - b)
		case OpSubGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_sub(a, b))
		case OpMulInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a * b)
		case OpMulFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a * b)
		case OpMulGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_mul(a, b))
		case OpDivInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			if b == 0 {
				panic("divide by zero")
			}
			vm.push(a / b)
		case OpDivFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a / b)
		case OpDivGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_div(a, b))
		case OpModInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			if b == 0 {
				panic("divide by zero")
			}
			vm.push(a % b)
		case OpModFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(math.Mod(a, b))
		case OpModGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_mod(a, b))
		case OpEqInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a == b)
		case OpEqFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a == b)
		case OpEqString:
			b := vm.pop().(string)
			a := vm.pop().(string)
			vm.push(a == b)
		case OpEqGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(deepEqual(a, b))
		case OpLtInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a < b)
		case OpLtFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a < b)
		case OpLtGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(compare(a, b, SYM_LT))
		case OpLeInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a <= b)
		case OpLeFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a <= b)
		case OpLeGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(compare(a, b, SYM_LE))
		case OpGtInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a > b)
		case OpGtFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a > b)
		case OpGtGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(compare(a, b, SYM_GT))
		case OpGeInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a >= b)
		case OpGeFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a >= b)
		case OpGeGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(compare(a, b, SYM_GE))
		case OpNeInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			vm.push(a != b)
		case OpNeFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(a != b)
		case OpNeString:
			b := vm.pop().(string)
			a := vm.pop().(string)
			vm.push(a != b)
		case OpNeGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(!deepEqual(a, b))
		case OpNot:
			a := vm.pop()
			vm.push(!asBool(a))
		case OpNegInt:
			a := vm.pop().(int)
			vm.push(-a)
		case OpNegFloat:
			a := vm.pop().(float64)
			vm.push(-a)
		case OpNegGeneric:
			a := vm.pop()
			switch v := a.(type) {
			case int:
				vm.push(-v)
			case float64:
				vm.push(-v)
			default:
				vmPool.Put(vm)
				return nil, fmt.Errorf("cannot negate type %T", a)
			}
		case OpPowInt:
			b := vm.pop().(int)
			a := vm.pop().(int)
			if b < 0 {
				vm.push(math.Pow(float64(a), float64(b)))
			} else if result, overflow := intPow(a, b); !overflow {
				vm.push(result)
			} else {
				vm.push(math.Pow(float64(a), float64(b)))
			}
		case OpPowFloat:
			b := vm.pop().(float64)
			a := vm.pop().(float64)
			vm.push(math.Pow(a, b))
		case OpPowGeneric:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_pow(a, b))
		case OpRange:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_range(a, b))
		case OpIn:
			b := vm.pop()
			a := vm.pop()
			vm.push(ev_in(a, b))
		case OpBitAnd:
			b := vm.vmPopInt()
			a := vm.vmPopInt()
			vm.push(a & b)
		case OpBitOr:
			b := vm.vmPopInt()
			a := vm.vmPopInt()
			vm.push(a | b)
		case OpBitXor:
			b := vm.vmPopInt()
			a := vm.vmPopInt()
			vm.push(a ^ b)
		case OpLShift:
			b := vm.vmPopInt()
			a := vm.vmPopInt()
			vm.push(a << b)
		case OpRShift:
			b := vm.vmPopInt()
			a := vm.vmPopInt()
			vm.push(a >> b)
		case OpStrLower:
			a := vm.pop().(string)
			vm.push(strings.ToLower(a))
		case OpStrUpper:
			a := vm.pop().(string)
			vm.push(strings.ToUpper(a))
		case OpStrTrim:
			a := vm.pop().(string)
			vm.push(strings.Trim(a, " \t\n\r"))
		case OpStrTrimLeft:
			a := vm.pop().(string)
			vm.push(strings.TrimLeft(a, " \t\n\r"))
		case OpStrTrimRight:
			a := vm.pop().(string)
			vm.push(strings.TrimRight(a, " \t\n\r"))
		case OpIndexGet:
			idx := vm.pop()
			container := vm.pop()
			vm.push(accessArray(vm.ident, container, idx))
		case OpArrayNew:
			count := int(instr.Arg1)
			ary := make([]any, count)
			for i := count - 1; i >= 0; i-- {
				ary[i] = vm.pop()
			}
			vm.push(ary)
		case OpMapNew:
			vm.push(make(map[string]any))
		case OpCallStd:
			// Not supported in v1
			vmPool.Put(vm)
			return nil, fmt.Errorf("CallStd not supported in bytecode v1")
		case OpCallUser:
			// Not supported in v1
			vmPool.Put(vm)
			return nil, fmt.Errorf("CallUser not supported in bytecode v1")
		case OpJumpIfFalse:
			cond := vm.pop()
			if !asBool(cond) {
				pc += int(int16(instr.Arg1)) - 1 // -1 because loop will increment
			}
		case OpJump:
			pc += int(int16(instr.Arg1)) - 1
		case OpLand:
			left := vm.peek()
			if !asBool(left) {
				vm.pop()
				vm.push(false)
				pc += int(int16(instr.Arg1)) - 1
			}
		case OpLor:
			left := vm.peek()
			if lstr, ok := left.(string); ok && lstr != "" {
				pc += int(int16(instr.Arg1)) - 1
			} else if asBool(left) {
				vm.pop()
				vm.push(true)
				pc += int(int16(instr.Arg1)) - 1
			}
		case OpToBool:
			vm.push(asBool(vm.pop()))
		case OpLorResult:
			a := vm.pop()
			if lstr, ok := a.(string); ok {
				vm.push(lstr)
			} else {
				vm.push(asBool(a))
			}
		case OpEnd:
			if vm.sp == 0 {
				result = nil
			} else {
				result = vm.pop()
			}
			if bcDebugExec {
				bcDumpExec(vm, pc, instr, result, nil)
			}
			vmPool.Put(vm)
			return result, nil
		default:
			vmPool.Put(vm)
			return nil, fmt.Errorf("unknown opcode %d", instr.Op)
		}

		if bcDebugExec {
			bcDumpExec(vm, pc, instr, nil, nil)
		}
	}

	vmPool.Put(vm)
	return nil, fmt.Errorf("bytecode did not end with OpEnd")
}

func (vm *ExprVM) push(v any) {
	if vm.sp >= len(vm.stack) {
		panic("stack overflow")
	}
	vm.stack[vm.sp] = v
	vm.sp++
}

func (vm *ExprVM) pop() any {
	if vm.sp <= 0 {
		panic("stack underflow")
	}
	vm.sp--
	v := vm.stack[vm.sp]
	vm.stack[vm.sp] = nil
	return v
}

func (vm *ExprVM) peek() any {
	if vm.sp <= 0 {
		panic("stack empty")
	}
	return vm.stack[vm.sp-1]
}

// vmPopInt coerces any integer type (int, uint, int64, uint64, etc.) to int.
// Used by bitwise opcodes that may receive C FFI constants as uint64/uint32/etc.
func (vm *ExprVM) vmPopInt() int {
	v := vm.pop()
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case int16:
		return int(val)
	case int8:
		return int(val)
	case uint:
		return int(val)
	case uint64:
		return int(val)
	case uint32:
		return int(val)
	case uint16:
		return int(val)
	case uint8:
		return int(val)
	case float64:
		return int(val)
	}
	panic(fmt.Errorf("expected integer, got %T", v))
}

func (vm *ExprVM) resolveIdent(name string) (any, bool) {
	// Try local first
	bin := bind_int(vm.fs, name)
	if bin < uint64(len(*vm.ident)) && (*vm.ident)[bin].declared && (*vm.ident)[bin].IName == name {
		return (*vm.ident)[bin].IValue, true
	}
	// Try global
	gbin := bind_int(vm.midentFS, name)
	if gbin < uint64(len(mident)) && mident[gbin].declared && mident[gbin].IName == name {
		return mident[gbin].IValue, true
	}
	// Try module constants (current namespace first, then USE chain)
	moduleConstantsLock.RLock()
	if constMap, exists := moduleConstants[vm.namespace]; exists {
		if val, found := constMap[name]; found {
			moduleConstantsLock.RUnlock()
			return val, true
		}
	}
	moduleConstantsLock.RUnlock()
	// Check USE chain for module constants
	if _, val, found := uc_match_constant(name); found {
		return val, true
	}
	// Try WITH ENUM context: resolve unqualified enum member names
	if vm.withEnumName != "" {
		fullEnumName := vm.namespace + "::" + vm.withEnumName
		if enumDef, exists := enum[fullEnumName]; exists {
			if memberVal, memberExists := enumDef.members[name]; memberExists {
				return memberVal, true
			}
		}
	}
	// Try enums
	ename := vm.namespace + "::" + name
	if enum[ename] != nil {
		return nil, true
	}
	return nil, false
}


// storeLocal replicates the full vset() semantics inside the VM.
func (vm *ExprVM) storeLocal(bin uint64, name string, val any) {
	needLock := vm.fs < 3 && atomic.LoadInt32(&concurrent_funcs) > 0
	if needLock {
		vlock.Lock()
		defer vlock.Unlock()
	}

	if bin >= uint64(len(*vm.ident)) {
		newIdent := make([]Variable, bin+identGrowthSize)
		copy(newIdent, *vm.ident)
		*vm.ident = newIdent
	}

	target := &(*vm.ident)[bin]

	if !target.declared {
		target.IName = name
		target.declared = true
		target.ITyped = false
		target.IKind = 0
	}

	if target.ITyped {
		var ok bool
		switch target.IKind {
		case kint:
			_, ok = val.(int)
		case kfloat:
			_, ok = val.(float64)
		case kbool:
			_, ok = val.(bool)
		case kuint:
			_, ok = val.(uint)
		case kuint64:
			_, ok = val.(uint64)
		case kint64:
			_, ok = val.(int64)
		case kbyte:
			_, ok = val.(uint8)
		case kstring:
			_, ok = val.(string)
		case kbigi:
			switch val.(type) {
			case uint, uint32, int, int64, uint64, float64, *big.Int, *big.Float, string, uint8:
				target.IValue.(*big.Int).Set(GetAsBigInt(val))
				ok = true
			}
		case kbigf:
			switch val.(type) {
			case uint, uint32, int, int64, uint64, float64, *big.Int, *big.Float, string, uint8:
				target.IValue.(*big.Float).Set(GetAsBigFloat(val))
				ok = true
			}
		case kmap:
			_, ok = val.(map[string]any)
		case kpointer:
			_, ok = val.(*CPointerValue)
			if !ok && val == nil {
				ok = true
			}
		case ksint:
			_, ok = val.([]int)
		case ksint64:
			_, ok = val.([]int64)
		case ksuint:
			_, ok = val.([]uint)
		case ksuint64:
			_, ok = val.([]uint64)
		case ksfloat:
			_, ok = val.([]float64)
		case ksstring:
			_, ok = val.([]string)
		case ksbool:
			_, ok = val.([]bool)
		case ksbyte:
			_, ok = val.([]uint8)
		case ksbigi:
			_, ok = val.([]*big.Int)
		case ksbigf:
			_, ok = val.([]*big.Float)
		case ksany:
			_, ok = val.([]any)
		case kdynamic:
			if target.IValue != nil {
				targetType := reflect.TypeOf(target.IValue)
				valueType := reflect.TypeOf(val)
				if valueType != nil && valueType.AssignableTo(targetType) {
					ok = true
				}
			}
		}

		if !ok {
			panic(fmt.Errorf("invalid assignation : to type [%T] of [%T]", target.IValue, val))
		}
	}

	if !target.ITyped || (target.IKind != kbigi && target.IKind != kbigf) {
		target.IValue = val
	}
}

func isExpressionStart(tok Token) bool {
	// Returns true if a phrase beginning with this token is an expression
	// that can be compiled to bytecode.
	switch tok.tokType {
	case NumericLiteral, StringLiteral, Identifier,
		O_Minus, O_Plus, SYM_Not, LParen, LeftSBrace, T_Map:
		return true
	default:
		return false
	}
}

// findAssignment scans tokens for an assignment operator (O_Assign, SYM_PLE, etc.)
// and returns the position of the first one found, or -1 if none.
// It also returns whether a comma exists in the tokens (for multi-assign detection).
func findAssignment(tks []Token) (pos int, hasComma bool) {
	for k, t := range tks {
		if t.tokType == O_Comma {
			hasComma = true
		}
		if t.tokType == O_Assign {
			return k, hasComma
		}
	}
	return -1, hasComma
}
