package main

import (
	"reflect"
	"testing"
)

func Test_typeOf(t *testing.T) {
	type args struct {
		val interface{}
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
			if got := typeOf(tt.args.val); got != tt.want {
				t.Errorf("typeOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_asBool(t *testing.T) {
	type args struct {
		val interface{}
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
			if got := asBool(tt.args.val); got != tt.want {
				t.Errorf("asBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_asInteger(t *testing.T) {
	type args struct {
		val interface{}
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := asInteger(tt.args.val); got != tt.want {
				t.Errorf("asInteger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_add(t *testing.T) {
	type args struct {
		val1 interface{}
		val2 interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := add(tt.args.val1, tt.args.val2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("add() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sub(t *testing.T) {
	type args struct {
		val1 interface{}
		val2 interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sub(tt.args.val1, tt.args.val2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sub() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mul(t *testing.T) {
	type args struct {
		val1 interface{}
		val2 interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mul(tt.args.val1, tt.args.val2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mul() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_div(t *testing.T) {
	type args struct {
		val1 interface{}
		val2 interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := div(tt.args.val1, tt.args.val2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("div() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mod(t *testing.T) {
	type args struct {
		val1 interface{}
		val2 interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mod(tt.args.val1, tt.args.val2); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_unaryMinus(t *testing.T) {
	type args struct {
		val interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unaryMinus(tt.args.val); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unaryMinus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deepEqual(t *testing.T) {
	type args struct {
		val1 interface{}
		val2 interface{}
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
			if got := deepEqual(tt.args.val1, tt.args.val2); got != tt.want {
				t.Errorf("deepEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_compare(t *testing.T) {
	type args struct {
		val1      interface{}
		val2      interface{}
		operation string
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
			if got := compare(tt.args.val1, tt.args.val2, tt.args.operation); got != tt.want {
				t.Errorf("compare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_compareInt(t *testing.T) {
	type args struct {
		val1      int
		val2      int
		operation string
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
			if got := compareInt(tt.args.val1, tt.args.val2, tt.args.operation); got != tt.want {
				t.Errorf("compareInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_compareFloat(t *testing.T) {
	type args struct {
		val1      float64
		val2      float64
		operation string
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
			if got := compareFloat(tt.args.val1, tt.args.val2, tt.args.operation); got != tt.want {
				t.Errorf("compareFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_asObjectKey(t *testing.T) {
	type args struct {
		key interface{}
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
			if got := asObjectKey(tt.args.key); got != tt.want {
				t.Errorf("asObjectKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_addMapMember(t *testing.T) {
	type args struct {
		evalfs uint64
		obj    string
		key    interface{}
		val    interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addMapMember(tt.args.evalfs, tt.args.obj, tt.args.key, tt.args.val)
		})
	}
}

func Test_addObjectMember(t *testing.T) {
	type args struct {
		evalfs uint64
		obj    string
		key    interface{}
		val    interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addObjectMember(tt.args.evalfs, tt.args.obj, tt.args.key, tt.args.val)
		})
	}
}

func Test_convertToInt(t *testing.T) {
	type args struct {
		ar interface{}
	}
	tests := []struct {
		name string
		args args
		want []int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertToInt(tt.args.ar); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertToFloat64(t *testing.T) {
	type args struct {
		ar interface{}
	}
	tests := []struct {
		name string
		args args
		want []float64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertToFloat64(tt.args.ar); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_accessVar(t *testing.T) {
	type args struct {
		evalfs  uint64
		varName string
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
			got, got1 := accessVar(tt.args.evalfs, tt.args.varName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accessVar() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("accessVar() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_accessField(t *testing.T) {
	type args struct {
		evalfs uint64
		obj    interface{}
		field  interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accessField(tt.args.evalfs, tt.args.obj, tt.args.field); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accessField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_slice(t *testing.T) {
	type args struct {
		v    interface{}
		from interface{}
		to   interface{}
	}
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slice(tt.args.v, tt.args.from, tt.args.to); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("slice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_arrayContains(t *testing.T) {
	type args struct {
		arr interface{}
		val interface{}
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
			if got := arrayContains(tt.args.arr, tt.args.val); got != tt.want {
				t.Errorf("arrayContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_callFunction(t *testing.T) {
	type args struct {
		evalfs uint64
		name   string
		args   []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantRes interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRes := callFunction(tt.args.evalfs, tt.args.name, tt.args.args); !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("callFunction() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}
