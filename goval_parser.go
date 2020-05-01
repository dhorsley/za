package main

import __yyfmt__ "fmt"

type yySymType struct {
	yys      int
	token    eToken
	expr     interface{}
	exprList []interface{}
	exprMap  map[string]interface{}
}

const LITERAL_NIL = 57346
const LITERAL_BOOL = 57347
const LITERAL_NUMBER = 57348
const LITERAL_STRING = 57349
const IDENT = 57350
const AND = 57351
const OR = 57352
const EQL = 57353
const NEQ = 57354
const LSS = 57355
const GTR = 57356
const LEQ = 57357
const GEQ = 57358
const SHL = 57359
const SHR = 57360
const BIT_NOT = 57361
const IN = 57362

var yyToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"LITERAL_NIL",
	"LITERAL_BOOL",
	"LITERAL_NUMBER",
	"LITERAL_STRING",
	"IDENT",
	"AND",
	"OR",
	"EQL",
	"NEQ",
	"LSS",
	"GTR",
	"LEQ",
	"GEQ",
	"SHL",
	"SHR",
	"BIT_NOT",
	"IN",
	"'|'",
	"'^'",
	"'&'",
	"'+'",
	"'-'",
	"'*'",
	"'/'",
	"'%'",
	"'!'",
	"'.'",
	"'['",
	"']'",
	"'('",
	"')'",
	"'{'",
	"'}'",
	"':'",
	"','",
}
var yyStatenames = [...]string{}

const yyEofCode = 1
const yyErrCode = 2
const yyInitialStackSize = 4

var yyExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const yyPrivate = 57344

const yyLast = 534

var yyAct = [...]int{

	44, 2, 85, 78, 76, 79, 77, 41, 69, 40,
	77, 37, 38, 46, 7, 6, 47, 48, 49, 50,
	51, 52, 53, 54, 55, 56, 57, 58, 59, 60,
	61, 62, 63, 64, 65, 66, 67, 68, 5, 70,
	72, 30, 31, 24, 25, 26, 27, 28, 29, 35,
	36, 4, 39, 32, 34, 33, 19, 20, 21, 22,
	23, 3, 37, 38, 81, 1, 39, 0, 0, 82,
	0, 0, 83, 0, 0, 43, 37, 38, 86, 39,
	87, 88, 0, 89, 0, 21, 22, 23, 0, 37,
	38, 0, 0, 94, 30, 31, 24, 25, 26, 27,
	28, 29, 35, 36, 0, 39, 32, 34, 33, 19,
	20, 21, 22, 23, 0, 37, 38, 75, 0, 0,
	0, 0, 92, 30, 31, 24, 25, 26, 27, 28,
	29, 35, 36, 0, 39, 32, 34, 33, 19, 20,
	21, 22, 23, 0, 37, 38, 0, 0, 0, 0,
	0, 80, 30, 31, 24, 25, 26, 27, 28, 29,
	35, 36, 0, 39, 32, 34, 33, 19, 20, 21,
	22, 23, 0, 37, 38, 0, 0, 73, 30, 31,
	24, 25, 26, 27, 28, 29, 35, 36, 0, 39,
	32, 34, 33, 19, 20, 21, 22, 23, 0, 37,
	38, 93, 30, 31, 24, 25, 26, 27, 28, 29,
	35, 36, 0, 39, 32, 34, 33, 19, 20, 21,
	22, 23, 0, 37, 38, 91, 30, 31, 24, 25,
	26, 27, 28, 29, 35, 36, 0, 39, 32, 34,
	33, 19, 20, 21, 22, 23, 0, 37, 38, 30,
	0, 24, 25, 26, 27, 28, 29, 35, 36, 0,
	39, 32, 34, 33, 19, 20, 21, 22, 23, 0,
	37, 38, 24, 25, 26, 27, 28, 29, 35, 36,
	0, 39, 32, 34, 33, 19, 20, 21, 22, 23,
	0, 37, 38, 24, 25, 26, 27, 28, 29, 35,
	36, 0, 39, 0, 34, 33, 19, 20, 21, 22,
	23, 0, 37, 38, 10, 11, 12, 13, 9, 26,
	27, 28, 29, 35, 36, 0, 39, 0, 0, 18,
	19, 20, 21, 22, 23, 16, 37, 38, 0, 17,
	0, 14, 0, 8, 0, 15, 0, 71, 24, 25,
	26, 27, 28, 29, 35, 36, 0, 39, 0, 0,
	33, 19, 20, 21, 22, 23, 0, 37, 38, 10,
	11, 12, 13, 9, 0, 0, 0, 0, 10, 11,
	12, 13, 9, 0, 18, 0, 0, 0, 0, 0,
	16, 0, 0, 18, 17, 0, 14, 0, 8, 16,
	15, 45, 0, 17, 0, 14, 90, 8, 0, 15,
	10, 11, 12, 13, 9, 0, 0, 0, 0, 35,
	36, 0, 39, 0, 0, 18, 19, 20, 21, 22,
	23, 16, 37, 38, 0, 17, 0, 14, 84, 8,
	0, 15, 24, 25, 26, 27, 28, 29, 35, 36,
	0, 39, 0, 0, 0, 19, 20, 21, 22, 23,
	0, 37, 38, 10, 11, 12, 13, 9, 0, 0,
	10, 11, 12, 13, 9, 0, 0, 0, 18, 0,
	0, 0, 0, 0, 16, 18, 0, 0, 17, 0,
	14, 16, 8, 74, 15, 17, 0, 14, 42, 8,
	0, 15, 10, 11, 12, 13, 9, 0, 39, 0,
	0, 0, 19, 20, 21, 22, 23, 18, 37, 38,
	0, 0, 0, 16, 0, 0, 0, 17, 0, 14,
	0, 8, 0, 15,
}
var yyPact = [...]int{

	498, -1000, 217, -1000, -1000, -1000, -1000, -1000, 498, -26,
	-1000, -1000, -1000, -1000, 466, 365, 498, 498, 498, 498,
	498, 498, 498, 498, 498, 498, 498, 498, 498, 498,
	498, 498, 498, 498, 498, 498, 498, 0, 310, 498,
	143, 459, -1000, -28, 217, -1000, -33, 114, 46, 46,
	46, 59, 59, 46, 46, 46, 306, 306, 402, 402,
	402, 402, 261, 240, 282, 431, 337, 488, 488, -1000,
	32, 406, -19, -1000, -1000, -32, -1000, 498, -1000, 498,
	498, -1000, 374, 193, -1000, -1000, 217, 85, 217, 169,
	-1000, -1000, 498, -1000, 217,
}
var yyPgo = [...]int{

	0, 65, 0, 61, 51, 38, 15, 14, 75, 13,
}
var yyR1 = [...]int{

	0, 1, 2, 2, 2, 2, 2, 2, 2, 2,
	3, 3, 3, 3, 3, 3, 3, 3, 4, 4,
	4, 4, 4, 4, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 6, 6, 6, 6, 6, 6, 7,
	7, 7, 7, 7, 7, 7, 7, 8, 8, 9,
	9,
}
var yyR2 = [...]int{

	0, 1, 1, 1, 1, 1, 1, 3, 3, 4,
	1, 1, 1, 1, 2, 3, 2, 3, 2, 3,
	3, 3, 3, 3, 2, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 2, 1,
	3, 4, 3, 6, 5, 5, 4, 1, 3, 3,
	5,
}
var yyChk = [...]int{

	-1000, -1, -2, -3, -4, -5, -6, -7, 33, 8,
	4, 5, 6, 7, 31, 35, 25, 29, 19, 24,
	25, 26, 27, 28, 11, 12, 13, 14, 15, 16,
	9, 10, 21, 23, 22, 17, 18, 30, 31, 20,
	-2, 33, 32, -8, -2, 36, -9, -2, -2, -2,
	-2, -2, -2, -2, -2, -2, -2, -2, -2, -2,
	-2, -2, -2, -2, -2, -2, -2, -2, -2, 8,
	-2, 37, -2, 34, 34, -8, 32, 38, 36, 38,
	37, 32, 37, -2, 32, 34, -2, -2, -2, -2,
	32, 32, 37, 32, -2,
}
var yyDef = [...]int{

	0, -2, 1, 2, 3, 4, 5, 6, 0, 39,
	10, 11, 12, 13, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 14, 0, 47, 16, 0, 0, 18, 24,
	38, 19, 20, 21, 22, 23, 25, 26, 27, 28,
	29, 30, 31, 32, 33, 34, 35, 36, 37, 40,
	0, 0, 42, 7, 8, 0, 15, 0, 17, 0,
	0, 41, 0, 0, 46, 9, 48, 0, 49, 0,
	45, 44, 0, 43, 50,
}
var yyTok1 = [...]int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 29, 3, 3, 3, 28, 23, 3,
	33, 34, 26, 24, 38, 25, 30, 27, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 37, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 31, 3, 32, 22, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 35, 21, 36,
}
var yyTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19, 20,
}
var yyTok3 = [...]int{
	0,
}

var yyErrorMessages = [...]struct {
	state int
	token int
	msg   string
}{}

/*	parser for yacc output	*/

var (
	yyErrorVerbose = false
)

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

type yyParser interface {
	Parse(yyLexer, uint64) (int,bool)
	Lookahead() int
}

type yyParserImpl struct {
	lval   yySymType
	stack  [yyInitialStackSize]yySymType
	char   int
	evalfs uint64
}

func (p *yyParserImpl) Lookahead() int {
	return p.char
}

func YyNewParser() yyParser {
	return &yyParserImpl{}
}

const yyFlag = -1000

// only used for error messages, no need for speed up:
func yyTokname(c int) string {
    d:=c-1
	if c >= 1 && d < len(yyToknames) {
		if yyToknames[d] != "" {
			return yyToknames[d]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s >= 0 && s < len(yyStatenames) {
		if yyStatenames[s] != "" {
			return yyStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func yyErrorMessage(state, lookAhead int) string {
	const TOKSTART = 4

	if !yyErrorVerbose {
		return "syntax error"
	}

	for _, e := range yyErrorMessages {
		if e.state == state && e.token == lookAhead {
			return "syntax error: " + e.msg
		}
	}

	res := "syntax error: unexpected " + yyTokname(lookAhead)

	// To match Bison, suggest at most four expected tokens.
	expected := make([]int, 0, 4)

	// Look for shiftable tokens.
	base := yyPact[state]
	lyytn := len(yyToknames)
    tmo:=TOKSTART-1
	for tok := TOKSTART; tmo < lyytn; tok++ {
		if n := base + tok; n >= 0 {
            if  n < yyLast && yyChk[yyAct[n]] == tok {
			    if len(expected) == cap(expected) {
				    return res
			    }
            }
			expected = append(expected, tok)
		}
	}

	if yyDef[state] == -2 {
		i := 0
		for yyExca[i] != -1 || yyExca[i+1] != state {
			i++
            i++
		}

		// Look for tokens that we accept or reduce.
		for i += 2; yyExca[i] >= 0; {
			tok := yyExca[i]
            i++
			if tok < TOKSTART || yyExca[i] == 0 {
				continue
			}
            i++
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}

		// If the default action is to accept or reduce, give up.
		if yyExca[i+1] != 0 {
			return res
		}
	}

	for i, tok := range expected {
		if i == 0 {
			res += ", expecting "
		} else {
			res += " or "
		}
		res += yyTokname(tok)
	}
	return res
}

func yylex1(lex yyLexer, lval *yySymType) (char, token int) {

	char = lex.Lex(lval)
	if char <= 0 {
		token = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		token = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			token = yyTok2[char-yyPrivate]
			goto out
		}
	}

out:
	if token == 0 {
		token = yyTok2[1] /* unknown char */
	}
	return char, token
}

func yyParse(yylex yyLexer, evalfs uint64) (int,bool) {
	res,ef:=YyNewParser().Parse(yylex, evalfs)
    return res,ef
}

func (yyrcvr *yyParserImpl) Parse(yylex yyLexer, evalfs uint64) (int,bool) {
	var yyn int
	var yyVAL yySymType
	var yyDollar []yySymType
	yyS := yyrcvr.stack[:]

    var ef bool = false
	var Nerrs int    /* number of errors */
	var Errflag int  /* error recovery flag */
	var yystate int
	yyrcvr.char = -1
	yyrcvr.evalfs = evalfs

	yytoken := -1 // yyrcvr.char translated into internal numbering
	yyp := -1
	goto yystack

ret1:
		yystate = -1
		yyrcvr.char = -1
		yytoken = -1
	return 1,ef

yystack:
	/* put a state and value onto the stack */

	yyp++
    ly:=len(yyS)
	if yyp >= ly {
		nyys := make([]yySymType, ly*3)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	yyn = yyPact[yystate]
	if yyn <= yyFlag {
		goto yydefault /* simple state */
	}
	if yyrcvr.char < 0 {
		yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
	}
	yyn += yytoken
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yytoken { /* valid shift */
		yyrcvr.char = -1
		yytoken = -1
		yyVAL = yyrcvr.lval
		yystate = yyn
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	}

yydefault:
	/* default state action */
	yyn = yyDef[yystate]
	if yyn == -2 {
		if yyrcvr.char < 0 {
			yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
		}

		/* look through exception table */
		xi := 0
		for {
			if yyExca[xi] == -1 {
                xi++
                if yyExca[xi] == yystate {
				    break
                }
			}
			xi++
		}
		for ; ; {
            xi++
            xi++
			yyn = yyExca[xi]
			if yyn < 0 || yyn == yytoken {
				break
			}
		}
		yyn = yyExca[xi+1]
		if yyn < 0 {
		    yystate = -1
		    yyrcvr.char = -1
		    yytoken = -1
	        return 0,ef
		}
	}
	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			yylex.Error(yyErrorMessage(yystate, yytoken))
			Nerrs++
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				yyn = yyPact[yyS[yyp].yys] + yyErrCode
				if yyn >= 0 && yyn < yyLast {
					yystate = yyAct[yyn] /* simulate a shift of "error" */
					if yyChk[yystate] == yyErrCode {
						goto yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yytoken == yyEofCode {
				goto ret1
			}
			yyrcvr.char = -1
			yytoken = -1
			goto yynewstate /* try again in the same state */
		}
	}

	/* reduction by production yyn */

	yynt := yyn
	yypt := yyp

	yyp -= yyR2[yyn]
	// yyp is now the index of $0. Perform the default action. Iff the
	// reduced production is ε, $1 is possibly out of range.
    ly=len(yyS)
    if yyp+1 >= ly {
		nyys := make([]yySymType, ly*3)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	yyn = yyR1[yyn]
	yyg := yyPgo[yyn]
	yyj := yyg + yyS[yyp].yys + 1

	if yyj >= yyLast {
		yystate = yyAct[yyg]
	} else {
		yystate = yyAct[yyj]
		if yyChk[yystate] != -yyn {
			yystate = yyAct[yyg]
		}
	}
	// dummy call; replaced with literal code
	switch yynt {

	case 1:
		yyDollar = yyS[yypt-1 : yypt+1]
		{
			yyVAL.expr = yyDollar[1].expr
			yylex.(*Lexer).result = yyVAL.expr
		}
	case 7:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = yyDollar[2].expr
		}
	case 8:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = callFunction(yyrcvr.evalfs, yyDollar[1].token.literal, []interface{}{})
		}
	case 9:
		yyDollar = yyS[yypt-4 : yypt+1]
		{
			yyVAL.expr = callFunction(yyrcvr.evalfs, yyDollar[1].token.literal, yyDollar[3].exprList)
		}
	case 10:
		yyDollar = yyS[yypt-1 : yypt+1]
		{
			yyVAL.expr = nil
		}
	case 11:
		yyDollar = yyS[yypt-1 : yypt+1]
		{
			yyVAL.expr = yyDollar[1].token.value
		}
	case 12:
		yyDollar = yyS[yypt-1 : yypt+1]
		{
			yyVAL.expr = yyDollar[1].token.value
		}
	case 13:
		yyDollar = yyS[yypt-1 : yypt+1]
		{
			yyVAL.expr = yyDollar[1].token.value
		}
	case 14:
		yyDollar = yyS[yypt-2 : yypt+1]
		{
			yyVAL.expr = []interface{}{}
		}
	case 15:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = yyDollar[2].exprList
		}
	case 16:
		yyDollar = yyS[yypt-2 : yypt+1]
		{
			yyVAL.expr = map[string]interface{}{}
		}
	case 17:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = yyDollar[2].exprMap
		}
	case 18:
		yyDollar = yyS[yypt-2 : yypt+1]
		{
			yyVAL.expr = unaryMinus(yyDollar[2].expr)
		}
	case 19:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = add(yyDollar[1].expr, yyDollar[3].expr)
		}
	case 20:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = sub(yyDollar[1].expr, yyDollar[3].expr)
		}
	case 21:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = mul(yyDollar[1].expr, yyDollar[3].expr)
		}
	case 22:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = div(yyDollar[1].expr, yyDollar[3].expr)
		}
	case 23:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = mod(yyDollar[1].expr, yyDollar[3].expr)
		}
	case 24: // COMPARATORS
		yyDollar = yyS[yypt-2 : yypt+1]
		{
			yyVAL.expr = !asBool(yyDollar[2].expr)
		}
	case 25:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = deepEqual(yyDollar[1].expr, yyDollar[3].expr)
		}
	case 26:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = !deepEqual(yyDollar[1].expr, yyDollar[3].expr)
		}
	case 27:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = compare(yyDollar[1].expr, yyDollar[3].expr, "<")
		}
	case 28:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = compare(yyDollar[1].expr, yyDollar[3].expr, ">")
		}
	case 29:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = compare(yyDollar[1].expr, yyDollar[3].expr, "<=")
		}
	case 30:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = compare(yyDollar[1].expr, yyDollar[3].expr, ">=")
		}
	case 31: // LOGICAL OPS
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = asBool(yyDollar[1].expr) && asBool(yyDollar[3].expr)
		}
	case 32:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = asBool(yyDollar[1].expr) || asBool(yyDollar[3].expr)
		}
	case 33: // BIT-WISE OPS
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = asInteger(yyDollar[1].expr) | asInteger(yyDollar[3].expr)
		}
	case 34:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = asInteger(yyDollar[1].expr) & asInteger(yyDollar[3].expr)
		}
	case 35:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = asInteger(yyDollar[1].expr) ^ asInteger(yyDollar[3].expr)
		}
	case 36: // BIT-SHIFTING
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			l := asInteger(yyDollar[1].expr)
			r := asInteger(yyDollar[3].expr)
			if r >= 0 {
				yyVAL.expr = l << uint(r)
			} else {
				yyVAL.expr = l >> uint(-r)
			}
		}
	case 37:
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			l := asInteger(yyDollar[1].expr)
			r := asInteger(yyDollar[3].expr)
			if r >= 0 {
				yyVAL.expr = l >> uint(r)
			} else {
				yyVAL.expr = l << uint(-r)
			}
		}
	case 38:
		yyDollar = yyS[yypt-2 : yypt+1]
		{
			yyVAL.expr = ^asInteger(yyDollar[2].expr)
		}
	case 39: // GET VARIABLE
		yyDollar = yyS[yypt-1 : yypt+1]
		{
			yyVAL.expr,ef = accessVar(yyrcvr.evalfs, yyDollar[1].token.literal)
		}
	case 40: // GET STRUCT FIELD
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = accessField(yyrcvr.evalfs, yyDollar[1].expr, yyDollar[3].token.literal)
		}
	case 41: // GET MAP/SLICE ELEMENT?
		yyDollar = yyS[yypt-4 : yypt+1]
		{
			yyVAL.expr = accessField(yyrcvr.evalfs, yyDollar[1].token.literal, yyDollar[3].expr)
		}
	case 42: // IN?
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.expr = arrayContains(yyDollar[3].expr, yyDollar[1].expr)
		}
	case 43: // ARRAY SLICE
		yyDollar = yyS[yypt-6 : yypt+1]
		{
			yyVAL.expr = slice(yyDollar[1].expr, yyDollar[3].expr, yyDollar[5].expr)
		}
	case 44: // ARRAY SLICE (NO START)
		yyDollar = yyS[yypt-5 : yypt+1]
		{
			yyVAL.expr = slice(yyDollar[1].expr, nil, yyDollar[4].expr)
		}
	case 45: // ARRAY SLICE (NO END)
		yyDollar = yyS[yypt-5 : yypt+1]
		{
			yyVAL.expr = slice(yyDollar[1].expr, yyDollar[3].expr, nil)
		}
	case 46: // ARRAY SLICE (NO START OR END)
		yyDollar = yyS[yypt-4 : yypt+1]
		{
			yyVAL.expr = slice(yyDollar[1].expr, nil, nil)
		}
	case 47: // STRUCT LITERAL?
		yyDollar = yyS[yypt-1 : yypt+1]
		{
			yyVAL.exprList = []interface{}{yyDollar[1].expr}
		}
	case 48: // APPEND TO LIST
		yyDollar = yyS[yypt-3 : yypt+1]
		{
			yyVAL.exprList = append(yyDollar[1].exprList, yyDollar[3].expr)
		}
	case 49: // MAP ELEMENT READ VALUE
		yyDollar = yyS[yypt-3 : yypt+1]
		{
		    yyVAL.exprMap = make(map[string]interface{})
            yyVAL.exprMap[asObjectKey(yyDollar[1].expr)] = yyDollar[3].expr
		}
	case 50:
		yyDollar = yyS[yypt-5 : yypt+1]
		{
            if yyDollar[1].exprList!=nil {
			    addObjectMember(yyrcvr.evalfs, yyDollar[1].token.literal, yyDollar[3].expr, yyDollar[5].expr)
            } else {
			    addMapMember(yyrcvr.evalfs, yyDollar[1].token.literal, yyDollar[3].expr, yyDollar[5].expr)
            }
		}
	}
	goto yystack /* stack new state and value */
}
