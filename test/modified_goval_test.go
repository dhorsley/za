package main

import (
	"reflect"
	"testing"
)

func TestNewEvaluator(t *testing.T) {
	tests := []struct {
		name string
		want *Evaluator
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewEvaluator(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewEvaluator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_Evaluate(t *testing.T) {
	type args struct {
		str    string
		evalfs uint64
	}
	tests := []struct {
		name       string
		e          *Evaluator
		args       args
		wantResult interface{}
		wantEf     bool
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Evaluator{}
			gotResult, gotEf, err := e.Evaluate(tt.args.str, tt.args.evalfs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluator.Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Evaluator.Evaluate() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
			if gotEf != tt.wantEf {
				t.Errorf("Evaluator.Evaluate() gotEf = %v, want %v", gotEf, tt.wantEf)
			}
		})
	}
}
