package main

import ()

// NewEvaluator creates a new evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluator is used to evaluate expression strings.
type Evaluator struct {
}

// ExpressionFunction can be called from within expressions.
// The returned object needs to have one of the following types: `nil`, `bool`, `int`, `float64`, `[]interface{}` or `map[string]interface{}`.
type ExpressionFunction = func(evalfs uint64,args ...interface{}) (interface{}, error)


func (e *Evaluator) Evaluate(str string, evalfs uint64) (result interface{}, ef bool, err error) {
	r,ef,n:=Evaluate(str, evalfs)
    return r,ef,n
}
