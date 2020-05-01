package main

import (
	"testing"
)

func TestFinish(t *testing.T) {
	interactive = 1
	finish(false, 0)
}

func TestGetAsFloat(t *testing.T) {
	var value float64
	var invalid bool

	value, invalid = GetAsFloat(int(3))
	if invalid {
		t.Fatalf("could not convert int to float.")
	}

	value, invalid = GetAsFloat(uint(3))
	if invalid {
		t.Fatalf("could not convert uint to float.")
	}

	value, invalid = GetAsFloat(float32(3.141))
	if invalid {
		t.Fatalf("could not convert float32 to float.")
	}

	value, invalid = GetAsFloat(float64(3.141))
	if invalid {
		t.Fatalf("could not convert float64 to float.")
	}

	value, invalid = GetAsFloat(int64(3))
	if invalid {
		t.Fatalf("could not convert int64 to float.")
	}

	value, invalid = GetAsFloat(int32(3))
	if invalid {
		t.Fatalf("could not convert int32 to float.")
	}

	value, invalid = GetAsFloat(uint32(3))
	if invalid {
		t.Fatalf("could not convert uint32 to float.")
	}

	value, invalid = GetAsFloat(uint64(3))
	if invalid {
		t.Fatalf("could not convert uint64 to float.")
	}

	value, invalid = GetAsFloat(string("3.141"))
	if invalid {
		t.Fatalf("could not convert string to float. got %v {%T}", value, value)
	}

	var nottastypi = []string{"not 3.141"}
	value, invalid = GetAsFloat(nottastypi)
	if invalid {
		t.Logf("check for invalid - should not convert string to float.")
	}

}

func TestGetAsInt32(t *testing.T) {
	var value int32
	var invalid bool

	value, invalid = GetAsInt32(int(3))
	if invalid {
		t.Fatalf("could not convert int to int32.")
	}

	value, invalid = GetAsInt32(uint(3))
	if invalid {
		t.Fatalf("could not convert uint to int32.")
	}

	value, invalid = GetAsInt32(float32(3.141))
	if invalid {
		t.Fatalf("could not convert float32 to int32.")
	}

	value, invalid = GetAsInt32(float64(3.141))
	if invalid {
		t.Fatalf("could not convert float64 to int32.")
	}

	value, invalid = GetAsInt32(int64(3))
	if invalid {
		t.Fatalf("could not convert int64 to int32.")
	}

	value, invalid = GetAsInt32(int32(3))
	if invalid {
		t.Fatalf("could not convert int32 to int32.")
	}

	value, invalid = GetAsInt32(uint32(3))
	if invalid {
		t.Fatalf("could not convert uint32 to int32.")
	}

	value, invalid = GetAsInt32(uint64(3))
	if invalid {
		t.Fatalf("could not convert uint64 to int32.")
	}

	value, invalid = GetAsInt32(string("3"))
	if invalid {
		t.Fatalf("could not convert string to int32. got %v {%T}", value, value)
	}

	var nottastypi = []string{"not 3.141"}
	value, invalid = GetAsInt32(nottastypi)
	if invalid {
		t.Logf("check for invalid - should not convert string to int32.")
	}

}

func TestGetAsInt(t *testing.T) {
	var value int
	var invalid bool

	value, invalid = GetAsInt(int(3))
	if invalid {
		t.Fatalf("could not convert int to int.")
	}

	value, invalid = GetAsInt(uint(3))
	if invalid {
		t.Fatalf("could not convert uint to int.")
	}

	value, invalid = GetAsInt(float32(3.141))
	if invalid {
		t.Fatalf("could not convert float32 to int.")
	}

	value, invalid = GetAsInt(float64(3.141))
	if invalid {
		t.Fatalf("could not convert float64 to int.")
	}

	value, invalid = GetAsInt(int64(3))
	if invalid {
		t.Fatalf("could not convert int64 to int.")
	}

	value, invalid = GetAsInt(int32(3))
	if invalid {
		t.Fatalf("could not convert int32 to int.")
	}

	value, invalid = GetAsInt(uint32(3))
	if invalid {
		t.Fatalf("could not convert uint32 to int.")
	}

	value, invalid = GetAsInt(uint64(3))
	if invalid {
		t.Fatalf("could not convert uint64 to int.")
	}

	value, invalid = GetAsInt(string("3"))
	if invalid {
		t.Fatalf("could not convert string to int. got %v {%T}", value, value)
	}

	var nottastypi = []string{"not 3.141"}
	value, invalid = GetAsInt(nottastypi)
	if invalid {
		t.Logf("check for invalid - should not convert string to int.")
	}

}

func TestEvalCrush(t *testing.T) {
	ident[0] = make([]Variable, VAR_CAP)
	lastbase = uint64(0)
	functionspaces[0] = []Phrase{}
	var fakeTokens []Token

	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: "("})
	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: "`big dogs, `"})
	fakeTokens = append(fakeTokens, Token{tokType: C_Plus, tokText: "+"})
	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: "`big dogs, landing on my face!`"})
	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: ")"})
	fakeTokens = append(fakeTokens, Token{tokType: EOL, tokText: ""})

	res, e := EvalCrush(0, fakeTokens, 0, len(fakeTokens)-1)
	t.Logf("Result : %#v\n", res)
	t.Logf("Error  : %v\n", e)
}

func TestEvalCrushRest(t *testing.T) {
	ident[0] = make([]Variable, VAR_CAP)
	lastbase = uint64(0)
	functionspaces[0] = []Phrase{}
	var fakeTokens []Token

	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: "("})
	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: "`Are you police? `"})
	fakeTokens = append(fakeTokens, Token{tokType: C_Plus, tokText: "+"})
	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: "`No ma'm, we're musicians.`"})
	fakeTokens = append(fakeTokens, Token{tokType: StringLiteral, tokText: ")"})
	fakeTokens = append(fakeTokens, Token{tokType: EOL, tokText: ""})

	res, e := EvalCrushRest(0, fakeTokens, 0)
	t.Logf("Result : %#v\n", res)
	t.Logf("Error  : %v\n", e)
}

func TestInSlice(t *testing.T) {
	if !InSlice(0, []int{3, 2, 1, 0}) {
		t.Fatalf("should have found a zero.")
	}
	if InSlice(0, []int{3, 2, 1}) {
		t.Fatalf("should not have found a zero.")
	}
}

func TestInStringSlice(t *testing.T) {
	if !InStringSlice("blah", []string{"a", "b", "c", "blah"}) {
		t.Fatalf("should have found blah.")
	}
	if InStringSlice("blah", []string{"bar", "foo"}) {
		t.Fatalf("should not have found blah.")
	}
}

func TestLookahead(t *testing.T) {
	functionspaces[0] = []Phrase{}
	prog := "if true; if false;;;;;; nop; else;; nop; endif; endif"
	parse("@test_lookahead_0", prog, 0)
	fnl, _ := fnlookup.lmget("@test_lookahead_0")
	elsefound, _, er := lookahead(fnl, 0, 0, 1, C_Else, []int{C_If}, []int{C_Endif})
	endfound, enddistance, er := lookahead(fnl, 0, 0, 0, C_Endif, []int{C_If}, []int{C_Endif})
	if er {
		t.Fatalf("lookahead returned an error.")
	}
	if elsefound {
		t.Fatalf("should not have stopped at the else.")
	}
	if !endfound {
		t.Fatalf("should have found an endif.")
	}
	if enddistance != 6 {
		t.Fatalf("found the wrong endif.")
	} // empty statements should not be included in this count
}

func TestGetNextFnSpace(t *testing.T) {
	nfs := GetNextFnSpace()
	if nfs == 0 {
		t.Fatalf("something went horribly wrong allocating a new function space id.")
	}
}

/*
func TestShowDef(t *testing.T) {

	prog := "define showTest();nop;enddef"
	sd_name := "test_showdef"

	parse(sd_name, prog, 0)

	ifs := fnlookup[sd_name]

	loc := GetNextFnSpace()
	numlookup[loc] = sd_name
	fnlookup[sd_name] = loc
	functionspaces[loc] = []Phrase{}
	functionArgs[loc] = []string{}

	cs := call_s{}
	cs.base = loc
	cs.caller = ifs
	cs.fs = sd_name
	callstack[loc] = cs
	Call(MODE_CALL, 0, 0, MODE_NEW, Phrase{}, loc)

	if !ShowDef("showTest") {
		t.Logf("should have displayed test routine.")
	}
	if ShowDef("notshowTest") {
		t.Logf("should not have displayed test routine.")
	}

}
*/

/*
func BenchmarkCall1(b *testing.B) {

    vcreatetable(0,VAR_CAP)
	ident[0] = make([]Variable, VAR_CAP)

	lastbase = uint64(0)

    // 4: Identifier
    // 28 C_Assign

    var fakeTokens1 []Token

    fakeTokens1 = append(fakeTokens1, Token{tokType: 4,  tokText:"a"        })
    fakeTokens1 = append(fakeTokens1, Token{tokType: 28,  tokText:"="       })
	fakeTokens1 = append(fakeTokens1, Token{tokType: 4,  tokText: "a+1"     })
    fakeTokens1 = append(fakeTokens1, Token{tokType: 90, tokText:""         })

    var p1 = Phrase{}
    copy(p1.Tokens,fakeTokens1)

    p1.Text="a =a+1"
    p1.Original="a=a+1"
    p1.TokenCount=4

	functionspaces[0] = []Phrase{}
    functionspaces[0]=append(functionspaces[0],p1)

	for n := 0; n < b.N; n++ {
        Call(MODE_CALL, 0, 0, MODE_NEW, p1, 0)
	}

}
*/

func BenchmarkCallwrappedEv(b *testing.B) {
	vcreatetable(0, VAR_CAP)
	ident[0] = make([]Variable, VAR_CAP)

	lastbase = uint64(0)

	var fakeTokens1 []Token

	fakeTokens1 = append(fakeTokens1, Token{tokType: 4, tokText: "a+1"})
	fakeTokens1 = append(fakeTokens1, Token{tokType: 90, tokText: ""})

	var p1 = Phrase{}
	copy(p1.Tokens, fakeTokens1)

	p1.Text = "a+1"
	p1.Original = "a+1"
	p1.TokenCount = 4

	functionspaces[0] = []Phrase{}
	functionspaces[0] = append(functionspaces[0], p1)

	exprCarton := ExpressionCarton{}
	exprCarton.text = "a+1"
	vset(0, "a", 0)

	for n := 0; n < b.N; n++ {
		wrappedEval(0, exprCarton, false)
	}

}

func BenchmarkCall3Ev(b *testing.B) {

	vcreatetable(0, VAR_CAP)
	ident[0] = make([]Variable, VAR_CAP)
	lastbase = uint64(0)
	vset(0, "a", 0)

	for n := 0; n < b.N; n++ {
		ev(0, "a+1", false)
	}

}

func BenchmarkCall4evalEv(b *testing.B) {
	vcreatetable(0, VAR_CAP)
	ident[0] = make([]Variable, VAR_CAP)
	lastbase = uint64(0)
	vset(0, "a", 0)
	for n := 0; n < b.N; n++ {
		eval.Evaluate("a+1", 0)
	}
}

func BenchmarkCall5add(b *testing.B) {
	vcreatetable(0, VAR_CAP)
	ident[0] = make([]Variable, VAR_CAP)
	lastbase = uint64(0)
	vset(0, "a", 0)
	for n := 0; n < b.N; n++ {
		add(0, 1)
	}
}

func BenchmarkCall6newlex(b *testing.B) {
	vcreatetable(0, VAR_CAP)
	ident[0] = make([]Variable, VAR_CAP)
	lastbase = uint64(0)
	vset(0, "a", 0)
	for n := 0; n < b.N; n++ {
		// lexer=NewLexer("a+1")
	}
}

func BenchmarkCall7newlp(b *testing.B) {
	vcreatetable(0, VAR_CAP)
	ident[0] = make([]Variable, VAR_CAP)
	lastbase = uint64(0)
	vset(0, "a", 0)
	for n := 0; n < b.N; n++ {
		// lexer=NewLexer("a+1")
		// YyNewParser().Parse(lexer, 0)
	}
}

/*
func Testpane_redef(t *testing.T) {
}
func TestCall(t *testing.T) {
// mode int, ifs uint64, base uint64, varmode int,inbound Phrase, csloc uint64, va ...interface{}) (endFunc bool) {
}
func TestfindTokenDelim(t *testing.T) {
// tokens []Token, delim int, start int) (pos int) {
}
func TestfindDelim(t *testing.T) {
// tokens []Token, delim string, start int) (pos int) {
}
*/

func Test_finish(t *testing.T) {
	type args struct {
		hard bool
		i    int
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finish(tt.args.hard, tt.args.i)
		})
	}
}

func Test_searchToken(t *testing.T) {
	type args struct {
		base  uint64
		start int
		end   int
		sval  string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchToken(tt.args.base, tt.args.start, tt.args.end, tt.args.sval); got != tt.want {
				t.Errorf("searchToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_lookahead(t *testing.T) {
	type args struct {
		fs         uint64
		startLine  int
		startlevel int
		endlevel   int
		term       int
		indenters  []int
		dedenters  []int
	}
	tests := []struct {
		name  string
		args  args
		want  bool
		want1 int
		want2 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := lookahead(tt.args.fs, tt.args.startLine, tt.args.startlevel, tt.args.endlevel, tt.args.term, tt.args.indenters, tt.args.dedenters)
			if got != tt.want {
				t.Errorf("lookahead() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("lookahead() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("lookahead() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func Test_pane_redef(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pane_redef()
		})
	}
}

func TestCall(t *testing.T) {
	type args struct {
		mode    int
		ifs     uint64
		base    uint64
		varmode int
		inbound Phrase
		csloc   uint64
		va      []interface{}
	}
	tests := []struct {
		name            string
		args            args
		wantEndFunc     bool
		wantBreakOut    int
		wantContinueOut int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEndFunc, gotBreakOut, gotContinueOut := Call(tt.args.mode, tt.args.ifs, tt.args.base, tt.args.varmode, tt.args.inbound, tt.args.csloc, tt.args.va...)
			if gotEndFunc != tt.wantEndFunc {
				t.Errorf("Call() gotEndFunc = %v, want %v", gotEndFunc, tt.wantEndFunc)
			}
			if gotBreakOut != tt.wantBreakOut {
				t.Errorf("Call() gotBreakOut = %v, want %v", gotBreakOut, tt.wantBreakOut)
			}
			if gotContinueOut != tt.wantContinueOut {
				t.Errorf("Call() gotContinueOut = %v, want %v", gotContinueOut, tt.wantContinueOut)
			}
		})
	}
}

func Test_bashCall(t *testing.T) {
	type args struct {
		ifs uint64
		s   string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bashCall(tt.args.ifs, tt.args.s)
		})
	}
}

func TestShowDef(t *testing.T) {
	type args struct {
		fn string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShowDef(tt.args.fn); got != tt.want {
				t.Errorf("ShowDef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findTokenDelim(t *testing.T) {
	type args struct {
		tokens []Token
		delim  int
		start  int
	}
	tests := []struct {
		name    string
		args    args
		wantPos int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotPos := findTokenDelim(tt.args.tokens, tt.args.delim, tt.args.start); gotPos != tt.wantPos {
				t.Errorf("findTokenDelim() = %v, want %v", gotPos, tt.wantPos)
			}
		})
	}
}

func Test_findDelim(t *testing.T) {
	type args struct {
		tokens []Token
		delim  string
		start  int
	}
	tests := []struct {
		name    string
		args    args
		wantPos int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotPos := findDelim(tt.args.tokens, tt.args.delim, tt.args.start); gotPos != tt.wantPos {
				t.Errorf("findDelim() = %v, want %v", gotPos, tt.wantPos)
			}
		})
	}
}
