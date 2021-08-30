package main

// we'll do something with this eventually....

//
// for compiling, we could either:
//  a) add a parse/compile type flag to the Eval/dparse parser.
//  b) write a fresh version that has the same structure but only emits code
//  c) something else?
// a) would work, but be messy. it would slow down normal interpretation, but
//     that probably wouldn't matter as most of the time it would just be
//     executed once in compile mode then not repeated.
// b) would be clean, but involve quite a bit of duplication.
// c) might require changing how we store and handle things like consts
//     and also converting our statement handling to pure expressions... so less likely.
//
// would also need to decide if we should use "registers" as defined below or stick
//  with a stack based vm, which would be a better fit for hijacking the current interpreter.
//  either is fine if we are going with option b/c above. stack-based probably better with
//  option a.

// additionally, we currently re-use a lot of built-in language features of go. for example.
//  we build Go structs dynamically instead of implementing our own type, and we use Go maps
//  and dynamic arrays for our variable storage.
//
// we may be able to keep co-opting some of this, but would be better to actually do some work
//  of our own here! ideally, we should try to figure out a tagged union implementation before
//  thinking about any of that.
//


import (
//    "io/ioutil"
	. "github.com/puzpuzpuz/xsync"
//    "sync"
//    "sync/atomic"
//    "runtime"
//    str "strings"
//    "unsafe"
)


type vm struct {
    reg [32]int64
    //
    // registers 0..1:
    // r0=
    //   sz  so  su  si . . . . . . . . . . . . : s 
    //     state-was-zero
    //     state-overflow
    //     state-underflow
    //     state-illegal
    //     state-timeout
    // r1= reserved
    // 
    // registers 2..25:
    //   ra3 ra2 ra1 ra0  : rah ral : ra
    //   rb3 rb2 rb1 rb0  : rbh rbl : rb
    //   rc3 rc2 rc1 rc0  : rch rcl : rc
    //   rd3 rd2 rd1 rd0  : rdh rdl : rd
    //   re3 re2 re1 re0  : reh rel : re
    //   rf3 rf2 rf1 rf0  : rfh rfl : rf
    // registers 26..31:
    //
    pc uint32
}

type instruction struct {
    opcode      uint8
    _pad1       [3]uint8
    operand1    int64
    operand2    int64
    _pad2       uint64
}

type value struct {
    // needs something like a union type that we don't have.
    // will have to think about this. we'll probably just
    // end up with a similar thing to what we have now.
    // it may be better for a first attempt, to just do something
    // best-fit with the existing parser for a hopefully quite
    // considerable speed up and worry about a technically better
    // solution later.
}

const (
    _NOP uint8 = iota
    _PUSH       // push an int64 to stack
    _PUSH_E     // push an expression to stack
    _POP        // pop from stack
    _POP_E      // pop an expression from stack
    _CALL       // implies a register set push
    _RET        // all RETx imply a register set pop
    _RETZ       // return if sz true
    _RETNZ      // return if sz false
    _LD         // load reg op1 with op2
    _CMP        // set sz true if op1 and op2 equal
    _INC        // increment op1
    _DEC        // decrement op1
    _ADD        // add op1+op2
    _SUB        // etc..
    _MUL        // 
    _DIV        // 
    _JP         // jump unconditionally
    _JPZ        // jump if sz true
    _JPNZ       // jump if sz false
    _HALT       // terminate vm
    _INT        // reserve a software interrupt opcode
    _LDIR       // load-increment-repeat
    _LDDR       // load-decrement-repeat
    _ILL        // illegal opcode
)

// operand types
// register, direct value, address, indirect access

// start spec'ing some interrupt routines
const (
    I_PRINT uint8 = iota        // value pointed to by reg rd
    I_USER
    I_GETCH                     // timeout in ra, returns in rd, sets state-timeout if exceeded
)


func bytecodeResize(loc int) {
    newar:=make([]bc_block,loc,loc)
    copy(newar,bytecode)
    bytecode=newar
}


type Expression struct {
}

func getNextExpr(t []Token) (e Expression,p int) {
    return e,p
}

var bclock = &RBMutex{}

func writeCode(loc uint32, va ...interface{}) {
    for vp:=range va {
        switch va[vp].(type) {
        case uint8:
            bytecode[loc].Block=append(bytecode[loc].Block,va[vp].(uint8))
        case interface{}:
        default:
        }
    }
}

// one option for translation would be statement based as below
// but this would probably require a lot of extra opcodes for special casing
// expressions as a higher level type.
// would probably be better to put the effort into avoiding this!

func translate_to_bytecode(loc uint32,source []Phrase) (translated bool) {
    bclock.Lock()
    // if len(source)>0 { translated=true }
    if len(bytecode)<=int(loc) {
        bytecodeResize(int(loc))
    }
    // pf("(translate) (with loc %d) bytecode collection size : %d\n",loc,len(bytecode))
    // pf("(translate) [#1]length of source : %d[#-]\n",len(source))
    for i:=0; i<len(source); i+=1 {
        // pf("(translate) source: %+v\n",source[i].Tokens)
        /*
        switch source[i].Tokens[0].tokType {
        case C_Nop:
            bytecode[loc].Block=append(bytecode[loc].Block,_NOP)
        case C_Exit:
            // + process exit code expr
            // + process exit message expr
            bytecode[loc].Block=append(bytecode[loc].Block,_HALT)
        case C_Return:
            // + process return expressions
            writeCode(loc,_RET)
        case C_Print:
            // + process print expressions
            count:=0
            var expr Expression
            for ke:=1; ke<len(source[i].Tokens); expr,ke=getNextExpr(source[i].Tokens[ke:]) {
                if ke==-1 { break }
                count+=1
                writeCode(loc,_PUSH_E,expr)
            }
            writeCode(loc,_PUSH,count,I_PRINT,_INT)
        default:
            translated=false
            break
        }
        */
    }
    // pf("(translate) completed with translation? %+v\n",translated)
    // if translated { pf("(translate) finished block :\n%#v\n",bytecode[loc].Block) }
    bclock.Unlock()
    return translated
}

func execute_bytecode(loc uint32) (retval_count uint8, endFunc bool) {
    return
}

