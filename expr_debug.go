package main

import (
	"fmt"
	"os"
	"strings"
)

func bcDumpCompile(phrase *Phrase, source string, code []Instr, pool []any, ok bool, reason string) {
	if !bcDebugCompile {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "[BC-COMPILE] line=%d source=%q\n", phrase.SourceLine, source)
	if ok {
		fmt.Fprintf(&sb, "  bytecode:\n%s\n", bcDisasm(code, pool))
		fmt.Fprintf(&sb, "  result: compiled ok (%d ops, %d constants)\n", len(code), len(pool))
	} else {
		fmt.Fprintf(&sb, "  result: fallback (%s)\n", reason)
	}
	os.Stderr.WriteString(sb.String())
}

func bcDumpExec(vm *ExprVM, pc int, instr Instr, result any, err error) {
	if !bcDebugExec {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "[BC-EXEC] pc=%04d %s\n", pc, bcDisasmInstr(instr, vm.pool))
	fmt.Fprintf(&sb, "  stack: %s\n", bcDumpStack(vm))
	if err != nil {
		fmt.Fprintf(&sb, "  result: error: %v\n", err)
	} else if result != nil {
		fmt.Fprintf(&sb, "  result: %v (type %T)\n", result, result)
	} else {
		fmt.Fprintf(&sb, "  result: nil\n")
	}
	os.Stderr.WriteString(sb.String())
}

func bcDisasm(code []Instr, pool []any) string {
	var sb strings.Builder
	for i, instr := range code {
		fmt.Fprintf(&sb, "    %04d  %s\n", i, bcDisasmInstr(instr, pool))
	}
	return sb.String()
}

func bcDisasmInstr(instr Instr, pool []any) string {
	var sb strings.Builder
	switch instr.Op {
	case OpLoadConstInt:
		if int(instr.Arg1) < len(pool) {
			fmt.Fprintf(&sb, "LoadConstInt %v", pool[instr.Arg1])
		} else {
			fmt.Fprintf(&sb, "LoadConstInt #%d", instr.Arg1)
		}
	case OpLoadConstFloat:
		if int(instr.Arg1) < len(pool) {
			fmt.Fprintf(&sb, "LoadConstFloat %v", pool[instr.Arg1])
		} else {
			fmt.Fprintf(&sb, "LoadConstFloat #%d", instr.Arg1)
		}
	case OpLoadConstString:
		if int(instr.Arg1) < len(pool) {
			fmt.Fprintf(&sb, "LoadConstString %q", pool[instr.Arg1])
		} else {
			fmt.Fprintf(&sb, "LoadConstString #%d", instr.Arg1)
		}
	case OpLoadLocal:
		fmt.Fprintf(&sb, "LoadLocal %d", instr.Arg1)
	case OpLoadGlobal:
		fmt.Fprintf(&sb, "LoadGlobal %d", instr.Arg1)
	case OpLoadIdent:
		if int(instr.Arg1) < len(pool) {
			fmt.Fprintf(&sb, "LoadIdent %q", pool[instr.Arg1])
		} else {
			fmt.Fprintf(&sb, "LoadIdent #%d", instr.Arg1)
		}
	case OpStoreLocal:
		if int(instr.Arg2) < len(pool) {
			fmt.Fprintf(&sb, "StoreLocal %d %q", instr.Arg1, pool[instr.Arg2])
		} else {
			fmt.Fprintf(&sb, "StoreLocal %d", instr.Arg1)
		}
	case OpPop:
		sb.WriteString("Pop")
	case OpDup:
		sb.WriteString("Dup")
	case OpAddInt:
		sb.WriteString("AddInt")
	case OpAddFloat:
		sb.WriteString("AddFloat")
	case OpAddString:
		sb.WriteString("AddString")
	case OpAddGeneric:
		sb.WriteString("AddGeneric")
	case OpSubInt:
		sb.WriteString("SubInt")
	case OpSubFloat:
		sb.WriteString("SubFloat")
	case OpSubGeneric:
		sb.WriteString("SubGeneric")
	case OpMulInt:
		sb.WriteString("MulInt")
	case OpMulFloat:
		sb.WriteString("MulFloat")
	case OpMulGeneric:
		sb.WriteString("MulGeneric")
	case OpDivInt:
		sb.WriteString("DivInt")
	case OpDivFloat:
		sb.WriteString("DivFloat")
	case OpDivGeneric:
		sb.WriteString("DivGeneric")
	case OpModInt:
		sb.WriteString("ModInt")
	case OpModFloat:
		sb.WriteString("ModFloat")
	case OpModGeneric:
		sb.WriteString("ModGeneric")
	case OpEqInt:
		sb.WriteString("EqInt")
	case OpEqFloat:
		sb.WriteString("EqFloat")
	case OpEqString:
		sb.WriteString("EqString")
	case OpEqGeneric:
		sb.WriteString("EqGeneric")
	case OpLtInt:
		sb.WriteString("LtInt")
	case OpLtFloat:
		sb.WriteString("LtFloat")
	case OpLtGeneric:
		sb.WriteString("LtGeneric")
	case OpLeInt:
		sb.WriteString("LeInt")
	case OpLeFloat:
		sb.WriteString("LeFloat")
	case OpLeGeneric:
		sb.WriteString("LeGeneric")
	case OpGtInt:
		sb.WriteString("GtInt")
	case OpGtFloat:
		sb.WriteString("GtFloat")
	case OpGtGeneric:
		sb.WriteString("GtGeneric")
	case OpGeInt:
		sb.WriteString("GeInt")
	case OpGeFloat:
		sb.WriteString("GeFloat")
	case OpGeGeneric:
		sb.WriteString("GeGeneric")
	case OpNeInt:
		sb.WriteString("NeInt")
	case OpNeFloat:
		sb.WriteString("NeFloat")
	case OpNeString:
		sb.WriteString("NeString")
	case OpNeGeneric:
		sb.WriteString("NeGeneric")
	case OpNot:
		sb.WriteString("Not")
	case OpNegInt:
		sb.WriteString("NegInt")
	case OpNegFloat:
		sb.WriteString("NegFloat")
	case OpNegGeneric:
		sb.WriteString("NegGeneric")
	case OpAnd:
		sb.WriteString("And")
	case OpOr:
		sb.WriteString("Or")
	case OpPow:
		sb.WriteString("Pow")
	case OpRange:
		sb.WriteString("Range")
	case OpIn:
		sb.WriteString("In")
	case OpBitAnd:
		sb.WriteString("BitAnd")
	case OpBitOr:
		sb.WriteString("BitOr")
	case OpBitXor:
		sb.WriteString("BitXor")
	case OpLShift:
		sb.WriteString("LShift")
	case OpRShift:
		sb.WriteString("RShift")
	case OpStrLower:
		sb.WriteString("StrLower")
	case OpStrUpper:
		sb.WriteString("StrUpper")
	case OpStrTitle:
		sb.WriteString("StrTitle")
	case OpStrTrimLeft:
		sb.WriteString("StrTrimLeft")
	case OpStrTrimRight:
		sb.WriteString("StrTrimRight")
	case OpIndexGet:
		sb.WriteString("IndexGet")
	case OpIndexSet:
		sb.WriteString("IndexSet")
	case OpFieldGet:
		sb.WriteString("FieldGet")
	case OpFieldSet:
		sb.WriteString("FieldSet")
	case OpArrayNew:
		fmt.Fprintf(&sb, "ArrayNew %d", instr.Arg1)
	case OpMapNew:
		sb.WriteString("MapNew")
	case OpCallStd:
		fmt.Fprintf(&sb, "CallStd %d %d", instr.Arg1, instr.Arg2)
	case OpCallUser:
		fmt.Fprintf(&sb, "CallUser %d %d", instr.Arg1, instr.Arg2)
	case OpJumpIfFalse:
		fmt.Fprintf(&sb, "JumpIfFalse %+d", int16(instr.Arg1))
	case OpJump:
		fmt.Fprintf(&sb, "Jump %+d", int16(instr.Arg1))
	case OpTernaryCond:
		fmt.Fprintf(&sb, "TernaryCond %+d", int16(instr.Arg1))
	case OpLoadConstSmallInt:
		fmt.Fprintf(&sb, "LoadConstSmallInt %d", int16(instr.Arg1))
	case OpLoadNil:
		sb.WriteString("LoadNil")
	case OpEnd:
		sb.WriteString("End")
	default:
		fmt.Fprintf(&sb, "Unknown(%d)", instr.Op)
	}
	return sb.String()
}

func bcDumpFold(op string, before []any, after any) {
	if !bcDebugFolding {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "[BC-FOLDING] op=%s before=[", op)
	for i, v := range before {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%v", v)
	}
	fmt.Fprintf(&sb, "] after=%v\n", after)
	os.Stderr.WriteString(sb.String())
}

func bcDumpStack(vm *ExprVM) string {
	if vm.sp == 0 {
		return "(empty)"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < vm.sp; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		v := vm.stack[i]
		if v == nil {
			sb.WriteString("nil")
		} else {
			fmt.Fprintf(&sb, "%v", v)
		}
	}
	sb.WriteString("]")
	return sb.String()
}
