package main

import (
	"fmt"
	"math/big"
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

// exprCompiler holds the state for compiling a single expression.
type exprCompiler struct {
	tokens    []Token
	pos       int
	fs        uint32
	ident     *[]Variable
	namespace string

	// output
	code  []Instr
	pool  []any
	hints []typeHint // type hint stack, parallel to vm stack during compilation
}

func compileExpr(tokens []Token, fs uint32, ident *[]Variable, namespace string) ([]Instr, []any, error) {
	c := &exprCompiler{
		tokens:    tokens,
		pos:       -1,
		fs:        fs,
		ident:     ident,
		namespace: namespace,
		code:      make([]Instr, 0, len(tokens)),
		pool:      make([]any, 0, 8),
		hints:     make([]typeHint, 0, 8),
	}
	// reserve pool[0] as nil
	c.pool = append(c.pool, nil)

	if err := c.expression(0); err != nil {
		return nil, nil, err
	}
	if c.pos+1 < len(c.tokens) {
		return nil, nil, fmt.Errorf("unexpected token %s after expression", tokNames[c.tokens[c.pos+1].tokType])
	}
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

func (c *exprCompiler) pushHint(h typeHint) {
	c.hints = append(c.hints, h)
}

func (c *exprCompiler) popHint() typeHint {
	if len(c.hints) == 0 {
		return hintUnknown
	}
	h := c.hints[len(c.hints)-1]
	c.hints = c.hints[:len(c.hints)-1]
	return h
	}

func (c *exprCompiler) peekHint() typeHint {
	if len(c.hints) == 0 {
		return hintUnknown
	}
	return c.hints[len(c.hints)-1]
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
		c.pushHint(hintString)
	case Identifier:
		return c.compileIdentifier(tok)
	case SYM_Not:
		if err := c.expression(24); err != nil {
			return err
		}
		c.emit(OpNot)
		c.popHint()
		c.pushHint(hintBool)
	case O_Minus:
		if err := c.expression(38); err != nil {
			return err
		}
		// unary minus: emit dedicated negation opcode based on operand type
		hint := c.popHint()
		switch hint {
		case hintFloat:
			c.emit(OpNegFloat)
			c.pushHint(hintFloat)
		case hintInt:
			c.emit(OpNegInt)
			c.pushHint(hintInt)
		default:
			c.emit(OpNegGeneric)
			c.pushHint(hintUnknown)
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
	case SYM_LAND, SYM_LOR:
		// The VM does not support short-circuiting or truthy/falsy value
		// semantics for &&/||.  dparse() handles both correctly.
		return fmt.Errorf("logical operators not supported in bytecode v1")
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
		idx := c.poolIndex(v)
		c.emit(OpLoadConstInt, idx)
		c.pushHint(hintInt)
	case float64:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstFloat, idx)
		c.pushHint(hintFloat)
	case *big.Int:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstInt, idx)
		c.pushHint(hintBigInt)
	case *big.Float:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstFloat, idx)
		c.pushHint(hintBigFloat)
	case bool:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstInt, idx)
		c.pushHint(hintBool)
	case string:
		idx := c.poolIndex(v)
		c.emit(OpLoadConstString, idx)
		c.pushHint(hintString)
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
			c.emit(OpLoadLocal, uint16(bin))
			c.pushHint(c.typeToHint((*c.ident)[bin].IValue))
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
		c.emit(OpLoadGlobal, uint16(gbin))
		c.pushHint(c.typeToHint(mident[gbin].IValue))
		return nil
	}

	// Fallback: runtime resolution
	idx := c.poolIndex(tok.tokText)
	if tok.bindpos < 65535 {
		c.emit(OpLoadIdent, idx, uint16(tok.bindpos+1))
	} else {
		c.emit(OpLoadIdent, idx)
	}
	c.pushHint(hintUnknown)
	return nil
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
	right := c.popHint()
	left := c.popHint()

	choose := func(intOp, floatOp, stringOp, genericOp OpCode) {
		if left == hintInt && right == hintInt {
			c.emit(intOp)
			c.pushHint(hintInt)
		} else if left == hintFloat && right == hintFloat {
			c.emit(floatOp)
			c.pushHint(hintFloat)
		} else if (left == hintInt || left == hintFloat) && (right == hintInt || right == hintFloat) {
			c.emit(floatOp)
			c.pushHint(hintFloat)
		} else if left == hintString && right == hintString && op == O_Plus {
			c.emit(stringOp)
			c.pushHint(hintString)
		} else {
			c.emit(genericOp)
			c.pushHint(hintUnknown)
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
        choose(OpDivGeneric, OpDivFloat, 0, OpDivGeneric)
	case O_Percent:
		choose(OpModInt, OpModGeneric, 0, OpModGeneric)
	}
}

func (c *exprCompiler) emitComparison(op int64) {
	right := c.popHint()
	left := c.popHint()

	choose := func(intOp, floatOp, stringOp, genericOp OpCode) {
		if left == hintInt && right == hintInt {
			c.emit(intOp)
		} else if left == hintFloat && right == hintFloat {
			c.emit(floatOp)
		} else if (left == hintInt || left == hintFloat) && (right == hintInt || right == hintFloat) {
			c.emit(floatOp)
		} else if left == hintString && right == hintString {
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
	c.pushHint(hintBool)
}

func (c *exprCompiler) emitLogical(op int64) {
	c.popHint()
	c.popHint()
	if op == SYM_LAND {
		c.emit(OpAnd)
	} else {
		c.emit(OpOr)
	}
	c.pushHint(hintBool)
}

func (c *exprCompiler) compileTernary() error {
	// condition is already on stack/hint
	c.popHint()

	// false branch
	if err := c.expression(0); err != nil {
		return err
	}
	_ = c.peekHint()

	if c.peek().tokType != SYM_COLON {
		return fmt.Errorf("expected : in ternary")
	}
	c.next()

	// true branch
	if err := c.expression(0); err != nil {
		return err
	}
	_ = c.peekHint()

	// We can't easily do short-circuit with stack-based VM without jumps.
	// For v1, evaluate both branches and then select. This is NOT short-circuit.
	// This is a correctness issue for side effects, but for v1 we accept it.
	// A proper implementation would use OpJumpIfFalse.

	// Actually, let's use a proper jump-based approach for ternary.
	// But that requires backpatching. Let's keep it simple: generic fallback.
	return fmt.Errorf("ternary not supported in bytecode v1")
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
	c.pushHint(hintSlice)
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
	c.popHint()
	c.popHint()
	c.emit(OpIndexGet)
	c.pushHint(hintUnknown)
	return nil
}

func (c *exprCompiler) compileFieldGet() error {
	// The VM does not support field access / method calls / enum members in v1.
	// dparse() handles these via accessFieldOrFunc() which dispatches to
	// struct fields (with renameSF), map keys, enum members, and stdlib methods.
	// Fallback to dparse() for all dot-access expressions.
	return fmt.Errorf("field access not supported in bytecode v1")
}

func (c *exprCompiler) poolIndex(v any) uint16 {
	for i, p := range c.pool {
		if p == v {
			return uint16(i)
		}
	}
	idx := len(c.pool)
	c.pool = append(c.pool, v)
	return uint16(idx)
}
