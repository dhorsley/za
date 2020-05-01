package main

import (
	"reflect"
	"testing"
)

func TestEvaluate(t *testing.T) {
	type args struct {
		str    string
		evalfs uint64
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
			gotResult, gotEf, err := Evaluate(tt.args.str, tt.args.evalfs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Evaluate() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
			if gotEf != tt.wantEf {
				t.Errorf("Evaluate() gotEf = %v, want %v", gotEf, tt.wantEf)
			}
		})
	}
}
