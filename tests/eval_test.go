// eval_test.go
package main

import (
    "math/big"
    "testing"
)

// var default_prectable [END_STATEMENTS] int8

func initDefaultPrecedence() {
    default_prectable[EOF]          =-1
    default_prectable[O_Assign]     =5          // L09
    default_prectable[O_Map]        =7
    default_prectable[O_Filter]     =9
    default_prectable[SYM_LOR]      =15         // L13
    default_prectable[C_Or]         =15         // L13
    default_prectable[SYM_LAND]     =15         // L12
    default_prectable[SYM_BAND]     =20         // L07
    default_prectable[SYM_BOR]      =20         // L07
    default_prectable[SYM_Caret]    =20         // L07
    default_prectable[SYM_LSHIFT]   =21         // L07
    default_prectable[SYM_RSHIFT]   =21         // L07
    default_prectable[O_Query]      =23 // tern // L14
    default_prectable[SYM_Tilde]    =25
    default_prectable[SYM_ITilde]   =25
    default_prectable[SYM_FTilde]   =25
    default_prectable[C_Is]         =25
    default_prectable[SYM_EQ]       =25         // L11
    default_prectable[SYM_NE]       =25         // L11
    default_prectable[SYM_LT]       =25         // L10
    default_prectable[SYM_GT]       =25         // L10
    default_prectable[SYM_LE]       =25         // L10
    default_prectable[SYM_GE]       =25         // L10
    default_prectable[C_In]         =27
    default_prectable[SYM_RANGE]    =29         // L08
    default_prectable[O_Plus]       =31         // L06
    default_prectable[O_Minus]      =31         // L06
    default_prectable[O_Divide]     =35         // L05
    default_prectable[O_Percent]    =35 // mod  // L05
    default_prectable[O_Multiply]   =35         // L05
    default_prectable[SYM_POW]      =37
    default_prectable[SYM_PP]       =45         // L02
    default_prectable[SYM_MM]       =45         // L02
}

// Test evaluating a bare numeric literal ("42") via ev(p, 1, ...),
// allowing for return types int, int64, or *big.Int.
func TestEvalLiteralViaEv(t *testing.T) {
    // 1) Register function‐space "main" → 1 so ev can resolve evalfs=1.
    fnlookup.lmset("main", 2)
    numlookup.lmset(2,"main")

    // 2) Initialize a parser with ident pointing to an empty []Variable slice,
    //    fs = 1, and namespace = "main".
    var locals []Variable
    p := &leparser{
        ident:     &locals,
        fs:        2,
        namespace: "main",
    }
    p.prectable=default_prectable

    // 3) Call ev(p, 2, "42") to evaluate the literal "42".
    res, err := ev(p, 2, "42")
    if err != nil {
        t.Fatalf(`ev(p, 2, "42") returned unexpected error: %v`, err)
    }

    // 4) ZA may return int, int64, or *big.Int for a numeric literal.
    switch v := res.(type) {
    case int:
        if v != 42 {
            t.Fatalf(`ev(p,2,"42") expected int(42), got int(%d)`, v)
        }
    case int64:
        if v != 42 {
            t.Fatalf(`ev(p,2,"42") expected int64(42), got int64(%d)`, v)
        }
    case *big.Int:
        if v.Cmp(big.NewInt(42)) != 0 {
            t.Fatalf(`ev(p,2,"42") expected *big.Int(42), got *big.Int(%v)`, v)
        }
    default:
        t.Fatalf(`ev(p,2,"42") returned unexpected type %T, value %v`, res, res)
    }
}

// Test evaluating a simple addition expression ("1 + 2") via ev(p, 1, ...),
// allowing for return types int, int64, or *big.Int.
func TestEvalAdditionViaEv(t *testing.T) {
    // 1) Ensure "main" → 2 exists in fnlookup (idempotent if already set).

    initDefaultPrecedence()
    fnlookup.lmset("main", 2)
    numlookup.lmset(2,"main")

    // 2) New parser instance with ident pointing to an empty []Variable slice.
    var locals []Variable
    p := &leparser{
        ident:     &locals,
        fs:        2,
        namespace: "main",
    }
    p.prectable=default_prectable

    // 3) Call ev(p, 2, "1 + 2").
    res, err := ev(p, 2, "1 + 2")
    if err != nil {
        t.Fatalf(`ev(p, 2, "1 + 2") returned unexpected error: %v`, err)
    }

    // 4) Assert that the result is int, int64, or *big.Int equal to 3.
    switch v := res.(type) {
    case int:
        if v != 3 {
            t.Fatalf(`ev(p,2,"1 + 2") expected int(3), got int(%d)`, v)
        }
    case int64:
        if v != 3 {
            t.Fatalf(`ev(p,2,"1 + 2") expected int64(3), got int64(%d)`, v)
        }
    case *big.Int:
        if v.Cmp(big.NewInt(3)) != 0 {
            t.Fatalf(`ev(p,2,"1 + 2") expected *big.Int(3), got *big.Int(%v)`, v)
        }
    default:
        t.Fatalf(`ev(p,2,"1 + 2") returned unexpected type %T, value %v`, res, res)
    }
}

