package lexer

import (
	"math/big"
)

func getAsBigInt(s string) *big.Int {
	var ri big.Int
	ri.SetString(s, 0)
	return &ri
}

func getAsBigFloat(s string) *big.Float {
	var r big.Float
	r.SetString(s)
	return &r
}
