
package main

type Stack []interface{}

/*
import (
    "sync"
)
var stacklock = &sync.RWMutex{}
var stack Stack
*/

func (s *Stack) IsEmpty() bool {
	return len(*s) == 0
}

func (s *Stack) Push(el interface{}) {
	*s = append(*s,el)
}
func (s *Stack) Fpop() (interface{}) {
    index := len(*s) - 1
    el := (*s)[index]
    *s = (*s)[:index]
    return el
}

func (s *Stack) Pop() (interface{},bool) {
	if s.IsEmpty() {
		return nil, false
	} else {
		index := len(*s) - 1
		el := (*s)[index]
		*s = (*s)[:index]
		return el, true
	}
}


