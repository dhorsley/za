package main

import (
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"reflect"
	"strings"
)

// typeHint tracks what we know about a value at compile time.
type typeHint uint8

const (
	hintUnknown typeHint = iota
	hintInt
	hintFloat
	hintString
	hintBool
	hintSlice
	hintMap
	hintStruct
	hintBigInt
	hintBigFloat
)

// compileValue tracks both the type hint and, for constants, the actual value.
// instrIdx is the index of the first instruction that produced this value, or -1
// if the value is not a simple constant load (e.g., result of a generic operation).
type compileValue struct {
	hint     typeHint
	constVal any // nil if not a compile-time constant
	instrIdx int // index of the first instruction for this value, or -1
}

// exprCompiler holds the state for compiling a single expression.
type exprCompiler struct {
	tokens    []Token
	pos       int
	fs        uint32
	ident     *[]Variable
	namespace string
	typeHints map[string]typeHint // parse-time type hints from VAR/def/for declarations

	// output
	code    []Instr
	pool    []any
	poolMap map[any]uint16
	values  []compileValue // value stack, parallel to vm stack during compilation
}

func compileExpr(tokens []Token, fs uint32, ident *[]Variable, typeHints map[string]typeHint, namespace string) ([]Instr, []any, error) {
	c := &exprCompiler{
		tokens:    tokens,
		pos:       -1,
		fs:        fs,
		ident:     ident,
		namespace: namespace,
		typeHints: typeHints,
		code:      make([]Instr, 0, len(tokens)),
		pool:      make([]any, 0, 8),
		poolMap:   make(map[any]uint16, 8),
		values:    make([]compileValue, 0, len(tokens)),
	}
	// reserve pool[0] as nil
	c.pool = append(c.pool, nil)
	c.poolMap[nil] = 0

	if err := c.expression(0); err != nil {
		return nil, nil, err
	}
	if c.pos+1 < len(c.tokens) {
		return nil, nil, fmt.Errorf("unexpected token %s after expression", tokNames[c.tokens[c.pos+1].tokType])
	}
	c.emit(OpEnd)
	return c.code, c.pool, nil
}

// compileSimpleAssign compiles a simple identifier = expr assignment to bytecode
// where the VM performs both the RHS evaluation and the store.
func compileSimpleAssign(tokens []Token, fs uint32, ident *[]Variable, typeHints map[string]typeHint, namespace string) ([]Instr, []any, error) {
	assignPos, hasComma := findAssignment(tokens)
	if assignPos < 0 || hasComma || assignPos == 0 || assignPos+1 >= len(tokens) {
		return nil, nil, fmt.Errorf("not a simple assignment")
	}
	if tokens[0].tokType != Identifier || assignPos != 1 {
		return nil, nil, fmt.Errorf("LHS is not a simple identifier")
	}

	c := &exprCompiler{
		tokens:    tokens,
		pos:       1, // position on '=' so expression() starts at RHS
		fs:        fs,
		ident:     ident,
		namespace: namespace,
		typeHints: typeHints,
		code:      make([]Instr, 0, len(tokens)),
		pool:      make([]any, 0, 8),
		poolMap:   make(map[any]uint16, 8),
		values:    make([]compileValue, 0, len(tokens)),
	}
	c.pool = append(c.pool, nil)
	c.poolMap[nil] = 0

	if err := c.expression(0); err != nil {
		return nil, nil, err
	}
	if c.pos+1 < len(c.tokens) {
		return nil, nil, fmt.Errorf("unexpected token %s after expression", tokNames[c.tokens[c.pos+1].tokType])
	}

	c.popValue() // pop RHS from value stack

	lhs := tokens[0]
	nameIdx := c.poolIndex(lhs.tokText)
	c.emit(OpStoreLocal, uint16(lhs.bindpos), nameIdx)
	c.emit(OpEnd)
	return c.code, c.pool, nil
}

func (c *exprCompiler) emit(op OpCode, args ...uint16) {
	instr := Instr{Op: op}
	if len(args) > 0 {
		instr.Arg1 = args[0]
	}
	if len(args) > 1 {
		instr.Arg2 = args[1]
	}
	c.code = append(c.code, instr)
}

func (c *exprCompiler) pushValue(v compileValue) {
	c.values = append(c.values, v)
}

func (c *exprCompiler) popValue() compileValue {
	if len(c.values) == 0 {
		return compileValue{hint: hintUnknown, instrIdx: -1}
	}
	v := c.values[len(c.values)-1]
	c.values = c.values[:len(c.values)-1]
	return v
}

func (c *exprCompiler) peekValue() compileValue {
	if len(c.values) == 0 {
		return compileValue{hint: hintUnknown, instrIdx: -1}
	}
	return c.values[len(c.values)-1]
}

// Backward-compatible hint wrappers (used by non-constant-aware paths)
func (c *exprCompiler) pushHint(h typeHint) {
	c.pushValue(compileValue{hint: h, constVal: nil, instrIdx: -1})
}

func (c *exprCompiler) popHint() typeHint {
	return c.popValue().hint
}

func (c *exprCompiler) peekHint() typeHint {
	return c.peekValue().hint
}

func (c *exprCompiler) next() Token {
	c.pos++
	if c.pos >= len(c.tokens) {
		return Token{tokType: EOF}
	}
	return c.tokens[c.pos]
}

func (c *exprCompiler) peek() Token {
	if c.pos+1 >= len(c.tokens) {
		return Token{tokType: EOF}
	}
	return c.tokens[c.pos+1]
}

func precedence(tt int64) int8 {
	if tt < 0 || tt >= int64(len(default_prectable)) {
		return PrecedenceInvalid
	}
	return default_prectable[tt]
}

func (c *exprCompiler) expression(minPrec int8) error {
	// prefix / nud
	tok := c.next()
	if err := c.nud(tok); err != nil {
		return err
	}

	// infix / led loop
	for {
		tok = c.peek()
		prec := precedence(tok.tokType)
		if prec < minPrec {
			break
		}
		c.next()
		if err := c.led(tok, prec); err != nil {
			return err
		}
	}
	return nil
}

func (c *exprCompiler) nud(tok Token) error {
	switch tok.tokType {
	case NumericLiteral:
		return c.compileLiteral(tok)
	case StringLiteral:
		idx := c.poolIndex(tok.tokText)
		c.emit(OpLoadConstString, idx)
		c.pushValue(compileValue{hint: hintString, constVal: tok.tokText, instrIdx: len(c.code) - 1})
	case Identifier:
		return c.compileIdentifier(tok)
	case T_Nil:
		c.emitLoadConst(nil)
		c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: len(c.code) - 1})
	case SYM_Not:
		startInstr := len(c.code)
		if err := c.expression(24); err != nil {
			return err
		}
		val := c.popValue()
		if val.constVal != nil {
			if result, ok := foldNot(val.constVal); ok {
				if bcDebugFolding {
					bcDumpFold("Not", []any{val.constVal}, result)
				}
				c.code = c.code[:startInstr]
				c.emitLoadConst(result)
				c.pushValue(compileValueFromResult(result, len(c.code)-1))
				return nil
			}
		}
		c.emit(OpNot)
		c.pushValue(compileValue{hint: hintBool, constVal: nil, instrIdx: -1})
	case O_Minus:
		startInstr := len(c.code)
		if err := c.expression(38); err != nil {
			return err
		}
		val := c.popValue()
		if val.constVal != nil {
			if result, ok := foldNeg(val.constVal); ok {
				if bcDebugFolding {
					bcDumpFold("Neg", []any{val.constVal}, result)
				}
				c.code = c.code[:startInstr]
				c.emitLoadConst(result)
				c.pushValue(compileValueFromResult(result, len(c.code)-1))
				return nil
			}
		}
		// unary minus: emit dedicated negation opcode based on operand type
		switch val.hint {
		case hintFloat:
			c.emit(OpNegFloat)
			c.pushValue(compileValue{hint: hintFloat, constVal: nil, instrIdx: -1})
		case hintInt:
			c.emit(OpNegInt)
			c.pushValue(compileValue{hint: hintInt, constVal: nil, instrIdx: -1})
		default:
			c.emit(OpNegGeneric)
			c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
		}
	case O_Plus:
		if err := c.expression(38); err != nil {
			return err
		}
		// unary plus is a no-op
	case LParen:
		if err := c.expression(0); err != nil {
			return err
		}
		if c.peek().tokType != RParen {
			return fmt.Errorf("expected )")
		}
		c.next()
	case LeftSBrace:
		return c.compileArrayLiteral()
	case T_Map:
		return c.compileMapLiteral()
	case O_Slc, O_Suc, O_Sst, O_Slt, O_Srt:
		startInstr := len(c.code)
		if err := c.expression(38); err != nil {
			return err
		}
		val := c.popValue()
		if val.constVal != nil {
			if result, ok := foldStringCase(tok.tokType, val.constVal); ok {
				if bcDebugFolding {
					bcDumpFold(tokNames[tok.tokType], []any{val.constVal}, result)
				}
				c.code = c.code[:startInstr]
				c.emitLoadConst(result)
				c.pushValue(compileValueFromResult(result, len(c.code)-1))
				return nil
			}
		}
		switch tok.tokType {
		case O_Slc:
			c.emit(OpStrLower)
		case O_Suc:
			c.emit(OpStrUpper)
		case O_Sst:
			c.emit(OpStrTrim)
		case O_Slt:
			c.emit(OpStrTrimLeft)
		case O_Srt:
			c.emit(OpStrTrimRight)
		}
		c.pushValue(compileValue{hint: hintString, constVal: nil, instrIdx: -1})
	default:
		return fmt.Errorf("cannot compile nud for token %s", tokNames[tok.tokType])
	}
	return nil
}

func (c *exprCompiler) led(tok Token, prec int8) error {
	switch tok.tokType {
	case O_Plus, O_Minus, O_Multiply, O_Divide, O_Percent:
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emitBinary(tok.tokType)
	case SYM_EQ, SYM_NE, SYM_LT, SYM_GT, SYM_LE, SYM_GE:
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emitComparison(tok.tokType)
	case SYM_POW:
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emitBinary(tok.tokType)
	case SYM_RANGE:
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emitRange()
	case C_In:
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emitIn()
	case SYM_BAND, SYM_BOR, SYM_Caret, SYM_LSHIFT, SYM_RSHIFT:
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emitBitwise(tok.tokType)
	case SYM_LAND:
		return c.compileLand(prec)
	case SYM_LOR, C_Or:
		return c.compileLor(prec)
	case O_Query:
		return c.compileTernary()
	case LeftSBrace:
		return c.compileIndexGet()
	case SYM_DOT:
		return c.compileFieldGet()
	case O_Assign:
		return fmt.Errorf("assignment not in expression context")
	default:
		return fmt.Errorf("cannot compile led for token %s", tokNames[tok.tokType])
	}
	return nil
}

func (c *exprCompiler) compileLiteral(tok Token) error {
	val := tok.tokVal
	switch v := val.(type) {
	case int:
		if v >= -32768 && v <= 32767 {
			c.emit(OpLoadConstSmallInt, uint16(int16(v)))
			c.pushValue(compileValue{hint: hintInt, constVal: v, instrIdx: len(c.code) - 1})
			return nil
		}
		idx := c.poolIndex(v)
		c.emit(OpLoadConstInt, idx)
		c.pushValue(compileValue{hint: hintInt, constVal: v, instrIdx: len(c.code) - 1})
	case float64:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstFloat, idx)
		c.pushValue(compileValue{hint: hintFloat, constVal: v, instrIdx: len(c.code) - 1})
	case *big.Int:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstInt, idx)
		c.pushValue(compileValue{hint: hintBigInt, constVal: v, instrIdx: len(c.code) - 1})
	case *big.Float:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstFloat, idx)
		c.pushValue(compileValue{hint: hintBigFloat, constVal: v, instrIdx: len(c.code) - 1})
	case bool:
		c.emit(OpLoadConstBool, boolToUint16(v))
		c.pushValue(compileValue{hint: hintBool, constVal: v, instrIdx: len(c.code) - 1})
	case string:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstString, idx)
		c.pushValue(compileValue{hint: hintString, constVal: v, instrIdx: len(c.code) - 1})
	case nil:
		c.emit(OpLoadNil)
		c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: len(c.code) - 1})
	default:
		return fmt.Errorf("unknown literal type %T", val)
	}
	return nil
}

func (c *exprCompiler) compileIdentifier(tok Token) error {
	// Handle builtin constants (true, false, nil, NaN, etc.)
	if tok.subtype == subtypeConst {
		return c.compileLiteral(tok)
	}

	// Check if it's a known local variable
	if c.ident != nil {
		bin := tok.bindpos
		if bin < uint64(len(*c.ident)) && (*c.ident)[bin].declared && (*c.ident)[bin].IName == tok.tokText {
			var hint typeHint
			if (*c.ident)[bin].ITyped {
				hint = kindOverrideToHint((*c.ident)[bin].Kind_override)
			} else {
				hint = c.typeToHint((*c.ident)[bin].IValue)
			}
			c.emit(OpLoadLocal, uint16(bin))
			c.pushValue(compileValue{hint: hint, constVal: nil, instrIdx: -1})
			return nil
		}
	}

	// Check if it's a global in mident
	var midentFS uint32
	if interactive {
		midentFS = 1
	} else {
		midentFS = 2
	}
	gbin := bind_int(midentFS, tok.tokText)
	if gbin < uint64(len(mident)) && mident[gbin].declared && mident[gbin].IName == tok.tokText {
		var hint typeHint
		if mident[gbin].ITyped {
			hint = kindOverrideToHint(mident[gbin].Kind_override)
		} else {
			hint = c.typeToHint(mident[gbin].IValue)
		}
		c.emit(OpLoadGlobal, uint16(gbin))
		c.pushValue(compileValue{hint: hint, constVal: nil, instrIdx: -1})
		return nil
	}

	// Fallback: parse-time type hints from VAR/def/for declarations
	if c.typeHints != nil {
		if hint, ok := c.typeHints[tok.tokText]; ok {
			idx := c.poolIndex(tok.tokText)
			if tok.bindpos < 65535 {
				c.emit(OpLoadIdent, idx, uint16(tok.bindpos+1))
			} else {
				c.emit(OpLoadIdent, idx)
			}
			c.pushValue(compileValue{hint: hint, constVal: nil, instrIdx: -1})
			return nil
		}
	}

	// Final fallback: runtime resolution
	idx := c.poolIndex(tok.tokText)
	if tok.bindpos < 65535 {
		c.emit(OpLoadIdent, idx, uint16(tok.bindpos+1))
	} else {
		c.emit(OpLoadIdent, idx)
	}
	c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
	return nil
}

// kindOverrideToHint maps a Kind_override type string to a typeHint.
func kindOverrideToHint(s string) typeHint {
	switch s {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return hintInt
	case "float64", "float32":
		return hintFloat
	case "string":
		return hintString
	case "bool":
		return hintBool
	case "[]int", "[]string", "[]float64", "[]bool", "[]any":
		return hintSlice
	default:
		if strings.HasPrefix(s, "[]") {
			return hintSlice
		}
		if strings.HasPrefix(s, "struct<") || strings.HasPrefix(s, "map[") {
			return hintStruct
		}
		return hintUnknown
	}
}

func (c *exprCompiler) typeToHint(v any) typeHint {
	if v == nil {
		return hintUnknown
	}
	switch v.(type) {
	case int:
		return hintInt
	case float64:
		return hintFloat
	case string:
		return hintString
	case bool:
		return hintBool
	case *big.Int:
		return hintBigInt
	case *big.Float:
		return hintBigFloat
	case []any, []int, []string, []float64:
		return hintSlice
	case map[string]any:
		return hintMap
	default:
		return hintUnknown
	}
}

func (c *exprCompiler) emitBinary(op int64) {
	right := c.popValue()
	left := c.popValue()

	// Try constant folding first
	if left.constVal != nil && right.constVal != nil && left.instrIdx >= 0 {
		if result, ok := foldBinary(op, left.constVal, right.constVal); ok {
			if bcDebugFolding {
				bcDumpFold(tokNames[op], []any{left.constVal, right.constVal}, result)
			}
			c.code = c.code[:left.instrIdx]
			c.emitLoadConst(result)
			c.pushValue(compileValueFromResult(result, len(c.code)-1))
			return
		}
	}

	choose := func(intOp, floatOp, stringOp, genericOp OpCode) {
		if left.hint == hintInt && right.hint == hintInt {
			c.emit(intOp)
			c.pushValue(compileValue{hint: hintInt, constVal: nil, instrIdx: -1})
		} else if left.hint == hintFloat && right.hint == hintFloat {
			c.emit(floatOp)
			c.pushValue(compileValue{hint: hintFloat, constVal: nil, instrIdx: -1})
	} else if (left.hint == hintInt || left.hint == hintFloat) && (right.hint == hintInt || right.hint == hintFloat) {
		c.emit(genericOp)
		c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
	} else if left.hint == hintString && right.hint == hintString && op == O_Plus {
			c.emit(stringOp)
			c.pushValue(compileValue{hint: hintString, constVal: nil, instrIdx: -1})
		} else {
			c.emit(genericOp)
			c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
		}
	}

	switch op {
	case O_Plus:
		choose(OpAddInt, OpAddFloat, OpAddString, OpAddGeneric)
	case O_Minus:
		choose(OpSubInt, OpSubFloat, 0, OpSubGeneric)
	case O_Multiply:
		choose(OpMulInt, OpMulFloat, 0, OpMulGeneric)
	case O_Divide:
		choose(OpDivInt, OpDivFloat, 0, OpDivGeneric)
	case O_Percent:
		choose(OpModInt, OpModFloat, 0, OpModGeneric)
	case SYM_POW:
		choose(OpPowInt, OpPowFloat, 0, OpPowGeneric)
	}
}

func (c *exprCompiler) emitComparison(op int64) {
	right := c.popValue()
	left := c.popValue()

	// Try constant folding first
	if left.constVal != nil && right.constVal != nil && left.instrIdx >= 0 {
		if result, ok := foldComparison(op, left.constVal, right.constVal); ok {
			if bcDebugFolding {
				bcDumpFold(tokNames[op], []any{left.constVal, right.constVal}, result)
			}
			c.code = c.code[:left.instrIdx]
			c.emitLoadConst(result)
			c.pushValue(compileValue{hint: hintBool, constVal: result, instrIdx: len(c.code) - 1})
			return
		}
	}

	choose := func(intOp, floatOp, stringOp, genericOp OpCode) {
		if left.hint == hintInt && right.hint == hintInt {
			c.emit(intOp)
		} else if left.hint == hintFloat && right.hint == hintFloat {
			c.emit(floatOp)
		} else if (left.hint == hintInt || left.hint == hintFloat) && (right.hint == hintInt || right.hint == hintFloat) {
			c.emit(genericOp)
		} else if left.hint == hintString && right.hint == hintString {
			c.emit(stringOp)
		} else {
			c.emit(genericOp)
		}
	}

	switch op {
	case SYM_EQ:
		choose(OpEqInt, OpEqFloat, OpEqString, OpEqGeneric)
	case SYM_NE:
		choose(OpNeInt, OpNeFloat, OpNeString, OpNeGeneric)
	case SYM_LT:
		choose(OpLtInt, OpLtFloat, 0, OpLtGeneric)
	case SYM_LE:
		choose(OpLeInt, OpLeFloat, 0, OpLeGeneric)
	case SYM_GT:
		choose(OpGtInt, OpGtFloat, 0, OpGtGeneric)
	case SYM_GE:
		choose(OpGeInt, OpGeFloat, 0, OpGeGeneric)
	default:
		c.emit(OpEqGeneric)
	}
	c.pushValue(compileValue{hint: hintBool, constVal: nil, instrIdx: -1})
}

func (c *exprCompiler) emitRange() {
	right := c.popValue()
	left := c.popValue()

	// Try constant folding
	if left.constVal != nil && right.constVal != nil && left.instrIdx >= 0 {
		if result, ok := foldRange(left.constVal, right.constVal); ok {
			if bcDebugFolding {
				bcDumpFold("Range", []any{left.constVal, right.constVal}, result)
			}
			c.code = c.code[:left.instrIdx]
			c.emitLoadConst(result)
			c.pushValue(compileValueFromResult(result, len(c.code)-1))
			return
		}
	}

	c.emit(OpRange)
	c.pushValue(compileValue{hint: hintSlice, constVal: nil, instrIdx: -1})
}

func (c *exprCompiler) emitIn() {
	right := c.popValue()
	left := c.popValue()

	// Try constant folding
	if left.constVal != nil && right.constVal != nil && left.instrIdx >= 0 {
		if result, ok := foldIn(left.constVal, right.constVal); ok {
			if bcDebugFolding {
				bcDumpFold("In", []any{left.constVal, right.constVal}, result)
			}
			c.code = c.code[:left.instrIdx]
			c.emitLoadConst(result)
			c.pushValue(compileValueFromResult(result, len(c.code)-1))
			return
		}
	}

	c.emit(OpIn)
	c.pushValue(compileValue{hint: hintBool, constVal: nil, instrIdx: -1})
}

func (c *exprCompiler) emitBitwise(op int64) {
	right := c.popValue()
	left := c.popValue()

	// Try constant folding (integer-only)
	if left.constVal != nil && right.constVal != nil && left.instrIdx >= 0 {
		if result, ok := foldBitwise(op, left.constVal, right.constVal); ok {
			if bcDebugFolding {
				bcDumpFold(tokNames[op], []any{left.constVal, right.constVal}, result)
			}
			c.code = c.code[:left.instrIdx]
			c.emitLoadConst(result)
			c.pushValue(compileValueFromResult(result, len(c.code)-1))
			return
		}
	}

	switch op {
	case SYM_BAND:
		c.emit(OpBitAnd)
	case SYM_BOR:
		c.emit(OpBitOr)
	case SYM_Caret:
		c.emit(OpBitXor)
	case SYM_LSHIFT:
		c.emit(OpLShift)
	case SYM_RSHIFT:
		c.emit(OpRShift)
	}
	c.pushValue(compileValue{hint: hintInt, constVal: nil, instrIdx: -1})
}

func (c *exprCompiler) compileTernary() error {
	// condition is already on stack from nud/left side

	// Emit OpJumpIfFalse with placeholder to jump to false branch.
	// Consumes the condition value.
	jumpFalseIdx := len(c.code)
	c.emit(OpJumpIfFalse, 0)

	// True branch: compile and leave result on stack.
	if err := c.expression(0); err != nil {
		return err
	}

	// After true branch, emit OpJump to skip false branch.
	jumpPastIdx := len(c.code)
	c.emit(OpJump, 0)

	// Patch OpJumpIfFalse to jump here (false branch start).
	c.code[jumpFalseIdx].Arg1 = uint16(int16(len(c.code) - jumpFalseIdx))

	// Consume the colon.
	if c.peek().tokType != SYM_COLON {
		return fmt.Errorf("expected : in ternary")
	}
	c.next()

	// False branch: compile and leave result on stack.
	if err := c.expression(0); err != nil {
		return err
	}

	// Patch OpJump to jump past false branch.
	c.code[jumpPastIdx].Arg1 = uint16(int16(len(c.code) - jumpPastIdx))

	c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
	return nil
}

func (c *exprCompiler) compileArrayLiteral() error {
	count := uint16(0)
	if c.peek().tokType != RightSBrace {
		for {
			if err := c.expression(0); err != nil {
				return err
			}
			count++
			if c.peek().tokType != O_Comma {
				break
			}
			c.next()
		}
	}
	if c.peek().tokType != RightSBrace {
		return fmt.Errorf("expected ]")
	}
	c.next()
	c.emit(OpArrayNew, count)
	c.pushValue(compileValue{hint: hintSlice, constVal: nil, instrIdx: -1})
	return nil
}

func (c *exprCompiler) compileMapLiteral() error {
	if c.peek().tokType != LParen {
		return fmt.Errorf("map literal without ()")
	}
	c.next()
	// For v1, we fall back to generic for map literals.
	return fmt.Errorf("map literal not supported in bytecode v1")
}

func (c *exprCompiler) compileIndexGet() error {
	if err := c.expression(0); err != nil {
		return err
	}
	if c.peek().tokType != RightSBrace {
		return fmt.Errorf("expected ]")
	}
	c.next()
	c.popValue()
	c.popValue()
	c.emit(OpIndexGet)
	c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
	return nil
}

func (c *exprCompiler) compileFieldGet() error {
	// The VM does not support field access / method calls / enum members in v1.
	// dparse() handles these via accessFieldOrFunc() which dispatches to
	// struct fields (with renameSF), map keys, enum members, and stdlib methods.
	// Fallback to dparse() for all dot-access expressions.
	return fmt.Errorf("field access not supported in bytecode v1")
}

func (c *exprCompiler) compileLand(prec int8) error {
	left := c.popValue()

	// Constant folding: if left is a known constant, we can sometimes skip the right side.
	if left.constVal != nil && left.instrIdx >= 0 {
		if !asBool(left.constVal) {
			// false && anything → false (short-circuit: right side never evaluated)
			if bcDebugFolding {
				bcDumpFold("Land", []any{left.constVal}, false)
			}
			c.code = c.code[:left.instrIdx]
			c.emit(OpLoadConstBool, boolToUint16(false))
			c.pushValue(compileValue{hint: hintBool, constVal: false, instrIdx: len(c.code) - 1})
			return nil
		}
		// true && X → X (coerce to bool, skip the jump machinery)
		if bcDebugFolding {
			bcDumpFold("Land", []any{left.constVal}, nil)
		}
		c.code = c.code[:left.instrIdx]
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emit(OpToBool)
		c.pushValue(compileValue{hint: hintBool, constVal: nil, instrIdx: -1})
		return nil
	}

	// Emit OpLand with placeholder offset.
	// OpLand peeks left; if falsy pushes false and jumps past right side.
	// If truthy falls through (stack empty, right side runs next).
	jumpIdx := len(c.code)
	c.emit(OpLand, 0)

	// Left was truthy: discard it, compile right side.
	c.emit(OpPop)
	if err := c.expression(prec + 1); err != nil {
		return err
	}

	// Coerce right side to boolean.
	c.emit(OpToBool)

	// Patch: Arg1 = distance from OpLand to here.
	c.code[jumpIdx].Arg1 = uint16(int16(len(c.code) - jumpIdx))

	c.pushValue(compileValue{hint: hintBool, constVal: nil, instrIdx: -1})
	return nil
}

func (c *exprCompiler) compileLor(prec int8) error {
	left := c.popValue()

	// Constant folding: if left is a known constant, we can sometimes skip the right side.
	if left.constVal != nil && left.instrIdx >= 0 {
		switch v := left.constVal.(type) {
		case string:
			if v != "" {
				// non-empty string || anything → string (short-circuit)
				if bcDebugFolding {
					bcDumpFold("Lor", []any{left.constVal}, v)
				}
				c.code = c.code[:left.instrIdx]
				idx := c.poolIndex(v)
				c.emit(OpLoadConstString, idx)
				c.pushValue(compileValue{hint: hintString, constVal: v, instrIdx: len(c.code) - 1})
				return nil
			}
			// "" || X → X (empty string is falsy, result is right side)
		default:
			if asBool(left.constVal) {
				// truthy non-string || anything → true (short-circuit)
				if bcDebugFolding {
					bcDumpFold("Lor", []any{left.constVal}, true)
				}
				c.code = c.code[:left.instrIdx]
				c.emit(OpLoadConstBool, boolToUint16(true))
				c.pushValue(compileValue{hint: hintBool, constVal: true, instrIdx: len(c.code) - 1})
				return nil
			}
			// falsy non-string || X → X (evaluate right)
		}
		// Left is falsy: fall through to compile right side without the jump machinery.
		if bcDebugFolding {
			bcDumpFold("Lor", []any{left.constVal}, nil)
		}
		c.code = c.code[:left.instrIdx]
		if err := c.expression(prec + 1); err != nil {
			return err
		}
		c.emit(OpLorResult)
		c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
		return nil
	}

	// Emit OpLor with placeholder offset.
	// OpLor peeks left; if truthy/string pushes result and jumps.
	// If falsy falls through (stack empty, right side runs next).
	jumpIdx := len(c.code)
	c.emit(OpLor, 0)

	// Left was falsy: discard it, compile right side.
	c.emit(OpPop)
	if err := c.expression(prec + 1); err != nil {
		return err
	}

	// Coerce right side: string passthrough, else asBool().
	c.emit(OpLorResult)

	// Patch: Arg1 = distance from OpLor to here.
	c.code[jumpIdx].Arg1 = uint16(int16(len(c.code) - jumpIdx))

	c.pushValue(compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1})
	return nil
}

func (c *exprCompiler) poolIndex(v any) uint16 {
	// Fast path: type-switch for common comparable constants.
	// avoids reflect.TypeOf() overhead for int, float64, string, bool,
	// *big.Int and *big.Float (all hashable as map keys).
	switch v.(type) {
	case int, float64, string, bool, *big.Int, *big.Float:
		if idx, ok := c.poolMap[v]; ok {
			return idx
		}
		idx := len(c.pool)
		c.pool = append(c.pool, v)
		c.poolMap[v] = uint16(idx)
		return uint16(idx)
	}
	// Slow path: linear scan with DeepEqual for non-hashable types (slices, maps)
	for i, p := range c.pool {
		if reflect.DeepEqual(p, v) {
			return uint16(i)
		}
	}
	idx := len(c.pool)
	c.pool = append(c.pool, v)
	return uint16(idx)
}

// emitLoadConst emits the appropriate load instruction for a constant value.
func (c *exprCompiler) emitLoadConst(v any) {
	switch val := v.(type) {
	case int:
		if val >= -32768 && val <= 32767 {
			c.emit(OpLoadConstSmallInt, uint16(int16(val)))
			return
		}
		idx := c.poolIndex(val)
		c.emit(OpLoadConstInt, idx)
	case float64:
		idx := c.poolIndex(val)
		c.emit(OpLoadConstFloat, idx)
	case string:
		idx := c.poolIndex(val)
		c.emit(OpLoadConstString, idx)
	case bool:
		c.emit(OpLoadConstBool, boolToUint16(val))
	case *big.Int:
		idx := c.poolIndex(val)
		c.emit(OpLoadConstInt, idx)
	case *big.Float:
		idx := c.poolIndex(val)
		c.emit(OpLoadConstFloat, idx)
	case nil:
		c.emit(OpLoadNil)
	default:
		// Fallback: generic load via int pool
		idx := c.poolIndex(v)
		c.emit(OpLoadConstInt, idx)
	}
}

// compileValueFromResult creates a compileValue from a computed result.
func compileValueFromResult(v any, instrIdx int) compileValue {
	switch val := v.(type) {
	case int:
		return compileValue{hint: hintInt, constVal: val, instrIdx: instrIdx}
	case float64:
		return compileValue{hint: hintFloat, constVal: val, instrIdx: instrIdx}
	case string:
		return compileValue{hint: hintString, constVal: val, instrIdx: instrIdx}
	case bool:
		return compileValue{hint: hintBool, constVal: val, instrIdx: instrIdx}
	case *big.Int:
		return compileValue{hint: hintBigInt, constVal: val, instrIdx: instrIdx}
	case *big.Float:
		return compileValue{hint: hintBigFloat, constVal: val, instrIdx: instrIdx}
	default:
		return compileValue{hint: hintUnknown, constVal: nil, instrIdx: -1}
	}
}

// foldBinary performs compile-time evaluation of binary operations on constants.
// It matches the runtime promotion rules used by ev_add, ev_sub, etc.
func foldBinary(op int64, left, right any) (any, bool) {
	switch op {
	case O_Plus:
		return foldAdd(left, right)
	case O_Minus:
		return foldSub(left, right)
	case O_Multiply:
		return foldMul(left, right)
	case O_Divide:
		return foldDiv(left, right)
	case O_Percent:
		return foldMod(left, right)
	case SYM_POW:
		return foldPow(left, right)
	}
	return nil, false
}

func foldAdd(left, right any) (any, bool) {
	switch l := left.(type) {
	case int:
		switch r := right.(type) {
		case int:
			sum, carry := bits.Add64(uint64(l), uint64(r), 0)
			// Check for signed overflow
			overflow := carry != 0 || (l >= 0 && int64(sum) < 0) || (l < 0 && int64(sum) > 0)
			if overflow {
				return nil, false
			}
			return int(sum), true
		case float64:
			return float64(l) + r, true
		case *big.Int:
			result := new(big.Int).SetInt64(int64(l))
			result.Add(result, r)
			return result, true
		case *big.Float:
			result := new(big.Float).SetInt64(int64(l))
			result.Add(result, r)
			return result, true
		}
	case float64:
		switch r := right.(type) {
		case int:
			return l + float64(r), true
		case float64:
			return l + r, true
		case *big.Float:
			result := new(big.Float).SetFloat64(l)
			result.Add(result, r)
			return result, true
		}
	case string:
		if r, ok := right.(string); ok {
			return l + r, true
		}
	case *big.Int:
		switch r := right.(type) {
		case *big.Int:
			return new(big.Int).Add(l, r), true
		case int:
			result := new(big.Int).Set(l)
			result.Add(result, new(big.Int).SetInt64(int64(r)))
			return result, true
		}
	case *big.Float:
		switch r := right.(type) {
		case *big.Float:
			return new(big.Float).Add(l, r), true
		case int:
			result := new(big.Float).Set(l)
			result.Add(result, new(big.Float).SetInt64(int64(r)))
			return result, true
		case float64:
			result := new(big.Float).Set(l)
			result.Add(result, new(big.Float).SetFloat64(r))
			return result, true
		}
	}
	return nil, false
}

func foldSub(left, right any) (any, bool) {
	switch l := left.(type) {
	case int:
		switch r := right.(type) {
		case int:
			diff, borrow := bits.Sub64(uint64(l), uint64(r), 0)
			// Check for signed overflow
			overflow := borrow != 0 || (l >= 0 && r < 0 && int64(diff) < 0) || (l < 0 && r > 0 && int64(diff) > 0)
			if overflow {
				return nil, false
			}
			return int(diff), true
		case float64:
			return float64(l) - r, true
		case *big.Int:
			result := new(big.Int).SetInt64(int64(l))
			result.Sub(result, r)
			return result, true
		case *big.Float:
			result := new(big.Float).SetInt64(int64(l))
			result.Sub(result, r)
			return result, true
		}
	case float64:
		switch r := right.(type) {
		case int:
			return l - float64(r), true
		case float64:
			return l - r, true
		case *big.Float:
			result := new(big.Float).SetFloat64(l)
			result.Sub(result, r)
			return result, true
		}
	case *big.Int:
		switch r := right.(type) {
		case *big.Int:
			return new(big.Int).Sub(l, r), true
		case int:
			result := new(big.Int).Set(l)
			result.Sub(result, new(big.Int).SetInt64(int64(r)))
			return result, true
		}
	case *big.Float:
		switch r := right.(type) {
		case *big.Float:
			return new(big.Float).Sub(l, r), true
		case int:
			result := new(big.Float).Set(l)
			result.Sub(result, new(big.Float).SetInt64(int64(r)))
			return result, true
		case float64:
			result := new(big.Float).Set(l)
			result.Sub(result, new(big.Float).SetFloat64(r))
			return result, true
		}
	}
	return nil, false
}

func foldMul(left, right any) (any, bool) {
	switch l := left.(type) {
	case int:
		switch r := right.(type) {
		case int:
			hi, lo := bits.Mul64(uint64(l), uint64(r))
			if hi != 0 {
				return nil, false
			}
			return int(lo), true
		case float64:
			return float64(l) * r, true
		case *big.Int:
			result := new(big.Int).SetInt64(int64(l))
			result.Mul(result, r)
			return result, true
		case *big.Float:
			result := new(big.Float).SetInt64(int64(l))
			result.Mul(result, r)
			return result, true
		}
	case float64:
		switch r := right.(type) {
		case int:
			return l * float64(r), true
		case float64:
			return l * r, true
		case *big.Float:
			result := new(big.Float).SetFloat64(l)
			result.Mul(result, r)
			return result, true
		}
	case *big.Int:
		switch r := right.(type) {
		case *big.Int:
			return new(big.Int).Mul(l, r), true
		case int:
			result := new(big.Int).Set(l)
			result.Mul(result, new(big.Int).SetInt64(int64(r)))
			return result, true
		}
	case *big.Float:
		switch r := right.(type) {
		case *big.Float:
			return new(big.Float).Mul(l, r), true
		case int:
			result := new(big.Float).Set(l)
			result.Mul(result, new(big.Float).SetInt64(int64(r)))
			return result, true
		case float64:
			result := new(big.Float).Set(l)
			result.Mul(result, new(big.Float).SetFloat64(r))
			return result, true
		}
	}
	return nil, false
}

func foldDiv(left, right any) (any, bool) {
	// Division by zero: do not fold, let runtime handle it
	switch r := right.(type) {
	case int:
		if r == 0 {
			return nil, false
		}
	case float64:
		if r == 0.0 {
			return nil, false
		}
	}

	switch l := left.(type) {
	case int:
		switch r := right.(type) {
		case int:
			return l / r, true
		case float64:
			return float64(l) / r, true
		}
	case float64:
		switch r := right.(type) {
		case int:
			return l / float64(r), true
		case float64:
			return l / r, true
		}
	case *big.Int:
		if r, ok := right.(*big.Int); ok {
			result := new(big.Float).SetInt(l)
			result.Quo(result, new(big.Float).SetInt(r))
			return result, true
		}
	case *big.Float:
		if r, ok := right.(*big.Float); ok {
			result := new(big.Float).Quo(l, r)
			return result, true
		}
	}
	return nil, false
}

func foldMod(left, right any) (any, bool) {
	// Modulo by zero: do not fold, let runtime handle it
	switch r := right.(type) {
	case int:
		if r == 0 {
			return nil, false
		}
	}

	switch l := left.(type) {
	case int:
		switch r := right.(type) {
		case int:
			return l % r, true
		case float64:
			return math.Mod(float64(l), r), true
		}
	case float64:
		switch r := right.(type) {
		case float64:
			return math.Mod(l, r), true
		case int:
			return math.Mod(l, float64(r)), true
		}
	}
	return nil, false
}

// foldEqual performs compile-time equality testing, handling big types and NaN
// correctly (unlike Go's == operator which does pointer comparison for big.Int).
func foldEqual(left, right any) (bool, bool) {
	if left == nil && right == nil {
		return true, true
	}
	if left == nil || right == nil {
		return false, true
	}
	switch l := left.(type) {
	case int:
		if r, ok := right.(int); ok {
			return l == r, true
		}
	case float64:
		if r, ok := right.(float64); ok {
			if math.IsNaN(l) && math.IsNaN(r) {
				return true, true
			}
			return l == r, true
		}
	case string:
		if r, ok := right.(string); ok {
			return l == r, true
		}
	case bool:
		if r, ok := right.(bool); ok {
			return l == r, true
		}
	case *big.Int:
		if r, ok := right.(*big.Int); ok {
			return l.Cmp(r) == 0, true
		}
	case *big.Float:
		if r, ok := right.(*big.Float); ok {
			return l.Cmp(r) == 0, true
		}
	}
	return false, false
}

// foldComparison performs compile-time comparison of constants.
func foldComparison(op int64, left, right any) (any, bool) {
	// Handle equality/inequality with proper type-aware comparison.
	// compareInt/compareFloat/compareString only support ordering (<, <=, >, >=).
	if op == SYM_EQ {
		if result, ok := foldEqual(left, right); ok {
			return result, true
		}
		return nil, false
	}
	if op == SYM_NE {
		if result, ok := foldEqual(left, right); ok {
			return !result, true
		}
		return nil, false
	}

	switch l := left.(type) {
	case int:
		switch r := right.(type) {
		case int:
			return compareInt(l, r, op), true
		case float64:
			return compareFloat(float64(l), r, op), true
		}
	case float64:
		switch r := right.(type) {
		case int:
			return compareFloat(l, float64(r), op), true
		case float64:
			return compareFloat(l, r, op), true
		}
	case string:
		if r, ok := right.(string); ok {
			return compareString(l, r, op), true
		}
	}
	return nil, false
}

// foldNeg performs compile-time negation.
func foldNeg(v any) (any, bool) {
	switch val := v.(type) {
	case int:
		if val == math.MinInt64 {
			return nil, false
		}
		return -val, true
	case float64:
		return -val, true
	case *big.Int:
		return new(big.Int).Neg(val), true
	case *big.Float:
		return new(big.Float).Neg(val), true
	}
	return nil, false
}

// foldNot performs compile-time logical negation.
func foldNot(v any) (any, bool) {
	if v == nil {
		return true, true
	}
	switch val := v.(type) {
	case bool:
		return !val, true
	case int:
		return val == 0, true
	case float64:
		return val == 0.0, true
	case string:
		return val == "", true
	case *big.Int:
		return val.Cmp(new(big.Int)) == 0, true
	case *big.Float:
		return val.Cmp(new(big.Float)) == 0, true
	}
	return nil, false
}

// foldPow performs compile-time power evaluation.
func foldPow(left, right any) (any, bool) {
	switch l := left.(type) {
	case int:
		switch r := right.(type) {
		case int:
			if r < 0 {
				return math.Pow(float64(l), float64(r)), true
			}
			if result, overflow := intPow(l, r); !overflow {
				return result, true
			}
			return math.Pow(float64(l), float64(r)), true
		case float64:
			return math.Pow(float64(l), r), true
		}
	case float64:
		switch r := right.(type) {
		case int:
			return math.Pow(l, float64(r)), true
		case float64:
			return math.Pow(l, r), true
		}
	}
	return nil, false
}

// intPow computes integer power with fast paths for common exponents.
// Returns (result, false) on success; (0, true) on overflow.
func intPow(base, exp int) (int, bool) {
	switch exp {
	case 0:
		return 1, false
	case 1:
		return base, false
	case 2:
		hi, lo := bits.Mul64(uint64(base), uint64(base))
		return int(lo), hi != 0
	case 3:
		hi, lo := bits.Mul64(uint64(base), uint64(base))
		if hi != 0 {
			return 0, true
		}
		hi2, lo2 := bits.Mul64(lo, uint64(base))
		return int(lo2), hi2 != 0
	}
	result := uint64(1)
	for exp > 0 {
		if exp&1 == 1 {
			hi, lo := bits.Mul64(result, uint64(base))
			if hi != 0 {
				return 0, true
			}
			result = lo
		}
		if exp >>= 1; exp > 0 {
			hi, lo := bits.Mul64(uint64(base), uint64(base))
			if hi != 0 {
				return 0, true
			}
			base = int(lo)
		}
	}
	return int(result), false
}

// foldRange performs compile-time range generation.
func foldRange(left, right any) (any, bool) {
	a, ok1 := left.(int)
	b, ok2 := right.(int)
	if !ok1 || !ok2 {
		return nil, false
	}
	if a > b {
		return []any{}, true
	}
	result := make([]any, b-a+1)
	for i := a; i <= b; i++ {
		result[i-a] = i
	}
	return result, true
}

// foldIn performs compile-time membership testing.
func foldIn(left, right any) (any, bool) {
	switch container := right.(type) {
	case string:
		if s, ok := left.(string); ok {
			return strings.Contains(container, s), true
		}
	case []any:
		for _, v := range container {
			if deepEqual(v, left) {
				return true, true
			}
		}
		return false, true
	case []int:
		if l, ok := left.(int); ok {
			for _, v := range container {
				if v == l {
					return true, true
				}
			}
			return false, true
		}
	case []string:
		if l, ok := left.(string); ok {
			for _, v := range container {
				if v == l {
					return true, true
				}
			}
			return false, true
		}
	}
	return nil, false
}

// foldBitwise performs compile-time bitwise operations on integers.
func foldBitwise(op int64, left, right any) (any, bool) {
	a, ok1 := left.(int)
	b, ok2 := right.(int)
	if !ok1 || !ok2 {
		return nil, false
	}
	switch op {
	case SYM_BAND:
		return a & b, true
	case SYM_BOR:
		return a | b, true
	case SYM_Caret:
		return a ^ b, true
	case SYM_LSHIFT:
		return a << b, true
	case SYM_RSHIFT:
		return a >> b, true
	}
	return nil, false
}

// foldStringCase performs compile-time string case transformations.
func foldStringCase(op int64, v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return nil, false
	}
	switch op {
	case O_Slc:
		return strings.ToLower(s), true
	case O_Suc:
		return strings.ToUpper(s), true
	case O_Sst:
		return strings.Trim(s, " \t\n\r"), true
	case O_Slt:
		return strings.TrimLeft(s, " \t\n\r"), true
	case O_Srt:
		return strings.TrimRight(s, " \t\n\r"), true
	}
	return nil, false
}

// Note: compareInt, compareFloat, and compareString are defined in eval_ops.go
// and are reused here for compile-time constant folding.

func boolToUint16(b bool) uint16 {
	if b {
		return 1
	}
	return 0
}
