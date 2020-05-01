package main

import (
	"reflect"
	"testing"
)

func TestVarLookup(t *testing.T) {
	type args struct {
		fs   uint64
		name string
	}
	tests := []struct {
		name  string
		args  args
		want  int
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := VarLookup(tt.args.fs, tt.args.name)
			if got != tt.want {
				t.Errorf("VarLookup() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("VarLookup() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_vcreatetable(t *testing.T) {
	type args struct {
		fs       uint64
		capacity int
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vcreatetable(tt.args.fs, tt.args.capacity)
		})
	}
}

func Test_vunset(t *testing.T) {
	type args struct {
		fs   uint64
		name string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vunset(tt.args.fs, tt.args.name)
		})
	}
}

func Test_vset(t *testing.T) {
	type args struct {
		fs    uint64
		name  string
		value interface{}
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
			if got := vset(tt.args.fs, tt.args.name, tt.args.value); got != tt.want {
				t.Errorf("vset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_vgetElement(t *testing.T) {
	type args struct {
		fs   uint64
		name string
		el   string
	}
	tests := []struct {
		name  string
		args  args
		want  interface{}
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := vgetElement(tt.args.fs, tt.args.name, tt.args.el)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("vgetElement() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("vgetElement() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_vsetElement(t *testing.T) {
	type args struct {
		fs    uint64
		name  string
		el    string
		value interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vsetElement(tt.args.fs, tt.args.name, tt.args.el, tt.args.value)
		})
	}
}

func Test_vget(t *testing.T) {
	type args struct {
		fs   uint64
		name string
	}
	tests := []struct {
		name  string
		args  args
		want  interface{}
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := vget(tt.args.fs, tt.args.name)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("vget() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("vget() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_isBool(t *testing.T) {
	type args struct {
		expr interface{}
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
			if got := isBool(tt.args.expr); got != tt.want {
				t.Errorf("isBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNumber(t *testing.T) {
	type args struct {
		expr interface{}
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
			if got := isNumber(tt.args.expr); got != tt.want {
				t.Errorf("isNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_interpolate(t *testing.T) {
	type args struct {
		fs uint64
		s  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := interpolate(tt.args.fs, tt.args.s); got != tt.want {
				t.Errorf("interpolate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_userDefEval(t *testing.T) {
	type args struct {
		ifs    uint64
		tokens []Token
	}
	tests := []struct {
		name  string
		args  args
		want  []Token
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := userDefEval(tt.args.ifs, tt.args.tokens)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("userDefEval() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("userDefEval() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_buildRhs(t *testing.T) {
	type args struct {
		ifs uint64
		rhs []Token
	}
	tests := []struct {
		name  string
		args  args
		want  []Token
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := buildRhs(tt.args.ifs, tt.args.rhs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRhs() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("buildRhs() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_ev(t *testing.T) {
	type args struct {
		fs       uint64
		ws       string
		interpol bool
	}
	tests := []struct {
		name       string
		args       args
		wantResult interface{}
		wantEf     bool
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotEf, err := ev(tt.args.fs, tt.args.ws, tt.args.interpol)
			if (err != nil) != tt.wantErr {
				t.Errorf("ev() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("ev() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
			if gotEf != tt.wantEf {
				t.Errorf("ev() gotEf = %v, want %v", gotEf, tt.wantEf)
			}
		})
	}
}

func Test_crushEvalTokens(t *testing.T) {
	type args struct {
		intoks []Token
	}
	tests := []struct {
		name string
		args args
		want ExpressionCarton
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := crushEvalTokens(tt.args.intoks); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("crushEvalTokens() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_tokenise(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name     string
		args     args
		wantToks []Token
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotToks := tokenise(tt.args.s); !reflect.DeepEqual(gotToks, tt.wantToks) {
				t.Errorf("tokenise() = %v, want %v", gotToks, tt.wantToks)
			}
		})
	}
}

func Test_wrappedEval(t *testing.T) {
	type args struct {
		fs       uint64
		expr     ExpressionCarton
		interpol bool
	}
	tests := []struct {
		name       string
		args       args
		wantResult ExpressionCarton
		wantEf     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotEf := wrappedEval(tt.args.fs, tt.args.expr, tt.args.interpol)
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("wrappedEval() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
			if gotEf != tt.wantEf {
				t.Errorf("wrappedEval() gotEf = %v, want %v", gotEf, tt.wantEf)
			}
		})
	}
}
