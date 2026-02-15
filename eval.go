package main

import (
    "context"
    "crypto/md5"
    "fmt"
    "io/ioutil"
    "math"
    "math/big"
    "net/http"
    "path/filepath"
    "reflect"
    "regexp"
    "strconv"
    str "strings"
    "sync"
    "sync/atomic"
    "unsafe"
)

func (p *leparser) reserved(token Token) any {
    panic(fmt.Errorf("statement names cannot be used as identifiers ([%s] %v)", tokNames[token.tokType], token.tokText))
}

func (p *leparser) Eval(fs uint32, toks []Token) (any, error) {

    l := len(toks)

    // short circuit pure numeric literals and const names
    if l == 1 {
        if toks[0].tokType == NumericLiteral {
            return toks[0].tokVal, nil
        }
        if toks[0].tokType == StringLiteral {
            return toks[0].tokText, nil
        }
        switch toks[0].subtype {
        case subtypeConst:
            return toks[0].tokVal, nil
        }
    }

    p.prectable = default_prectable
    p.fs = fs
    p.tokens = toks
    p.len = int16(l)
    p.pos = -1

    // pf("\n(eval) about to call dparse() with : %#v\n",toks)
    return p.dparse(0, false)
}

type leparser struct {
    tokens []Token     // the thing getting evaluated
    ident  *[]Variable // where are the local variables at?
    prev   Token       // bodge for post-fix operations
    prev2  Token       //   and the same for assignment
    fs     uint32      // working function space
    mident uint32      // fs of main() (1 or 2 depending on interactive mode)
    // @note: mident is necessary to say whether globals are stored under fs #1 or #2
    ctx           context.Context
    len           int16                // assigned length to save calling len() during parsing
    line          int16                // shadows lexer source line
    pc            int16                // shadows program counter (pc)
    pos           int16                // distance through parse
    prectable     [END_STATEMENTS]int8 // precedence lookup table
    namespace     string               // optional namespace attached to next 2 tokens
    namespacing   bool                 // pending namespace completion?
    namespace_pos int16                // token position of namespace start

    std_call    bool // if a call to stdlib has been made
    std_faulted bool // and if it faulted.

    in_range bool // currently in an eval calculating array ranges
    rangelen int  // length of array referenced in accessArray

    interpolating bool
    hard_fault    bool   // stop error bypass in fallback mode
    kind_override string // when self has been created, this bears the struct type.

    inside_with_struct bool
    inside_with_enum   bool
    with_struct_name   string
    with_enum_name     string
}

func (p *leparser) next() Token {
    p.prev2 = p.prev
    if p.pos >= 0 {
        p.prev = p.tokens[p.pos]
    }
    p.pos += 1
    return p.tokens[p.pos]
}

func (p *leparser) peek() Token {
    if p.pos+1 == p.len {
        return Token{tokType: EOF}
    }
    return p.tokens[p.pos+1]
}

func (p *leparser) dparse(prec int8, skip bool) (left any, err error) {

    // pf("[dparse] recv expr : %+v\n",p.tokens)
    //
    // Add recover() for ?? operator error routing
    defer func() {
        if r := recover(); r != nil {
            var errVal error
            var source string

            if e, ok := r.(error); ok {
                errVal = e
                if str.Contains(e.Error(), "?? operator failure") {
                    source = "try_operator"
                } else {
                    source = "evaluator"
                }
            } else {
                // Convert non-error panics to errors
                errVal = fmt.Errorf("%v", r)
                source = "evaluator"
            }

            // Check if this should be handled as an exception
            shouldConvertToException := false

            // Case 1: ?? operator failure (always convert to exception)
            if str.Contains(errVal.Error(), "?? operator failure") {
                shouldConvertToException = true
            }

            // Case 2: Error style mode is set to convert panics to exceptions
            errorStyleLock.RLock()
            currentErrorStyle := errorStyleMode
            errorStyleLock.RUnlock()

            if currentErrorStyle == ERROR_STYLE_EXCEPTION || currentErrorStyle == ERROR_STYLE_MIXED {
                shouldConvertToException = true
            }

            // Case 3: Auto-convert to exception if inside a try block
            if !shouldConvertToException {
                calllock.RLock()
                isTryBlock := calltable[p.fs].isTryBlock
                defaultCategory := calltable[p.fs].defaultExceptionCategory
                calllock.RUnlock()

                if isTryBlock {
                    shouldConvertToException = true
                    // Store the try block's default category for later use
                    if defaultCategory != nil {
                        // We'll use this category instead of the default "error" or "panic"
                        // This will be handled in the exception creation logic below
                    }
                }
            }

            if shouldConvertToException {
                // Convert to exception for try/catch handling
                var category any
                var message string

                message = errVal.Error()

                // Extract category from ?? operator error message if possible
                if str.Contains(message, "?? operator failure:") {
                    parts := str.Split(message, " -> ")
                    if len(parts) > 1 {
                        category = parts[1]
                        message = parts[0]
                    }
                } else {
                    // Check if we're in a try block with a default category
                    calllock.RLock()
                    defaultCategory := calltable[p.fs].defaultExceptionCategory
                    calllock.RUnlock()

                    if defaultCategory != nil {
                        // Use try block's default category
                        category = defaultCategory
                    } else {
                        // This is a regular panic - set category to "error" (not "panic" for auto-conversion)
                        category = "error"
                    }
                }

                // Create exception info with corrected line number (similar to C_Throw)
                stackTraceCopy := generateStackTrace(calltable[p.fs].fs, p.fs, p.line+1)
                excInfo := &exceptionInfo{
                    category:   category,
                    message:    message,
                    line:       int(p.line) + 1,
                    function:   calltable[p.fs].fs,
                    fs:         p.fs,
                    stackTrace: stackTraceCopy,
                    source:     source,
                }

                // Set the exception state atomically
                atomic.StorePointer(&calltable[p.fs].activeException, unsafe.Pointer(excInfo))

                // Don't re-panic - let the main execution loop detect the active exception
                // and route it to try/catch blocks
                return
            } else {
                // Let the error propagate normally through the return path
                // This will go through the main execution loop's enhanced error handling
                err = errVal
                left = nil
                return
            }
        }
    }()

    // @note: skip allows expression to be parsed without error in order to skip
    // past redundant phrases. not ideal, but okay for now.

    // pf("\ndparse query with fs #%d : spos %v : %#v\n",p.fs,p.pos,p.tokens)

    if skip {
        brace_level := 0
    skiploop1:
        for {
            switch p.peek().tokType {
            case LParen:
                brace_level += 1
            case RParen:
                if brace_level == 0 {
                    // pf("[skip breaking on token %v] ",tokNames[p.peek().tokType])
                    break skiploop1
                }
                brace_level -= 1
            case O_Comma, SYM_COLON, EOF:
                // pf("[skip breaking on token %v] ",tokNames[p.peek().tokType])
                break skiploop1
            }
            // pf("[skip token %+v] ",p.peek())
            p.next()
        }
        return left, err
    }

    // inlined next() manually:
    p.prev2 = p.prev
    if p.pos >= 0 {
        p.prev = p.tokens[p.pos]
    }
    p.pos += 1

    if p.pos >= p.len {
        return left, err
    }

    ct := &p.tokens[p.pos]

    if p.prectable[ct.tokType] == PrecedenceInvalid {
        panic(fmt.Errorf("Token '%s' is not allowed in expressions", ct.tokText))
    }

    // unaries
    switch (*ct).tokType {

    case O_Comma, SYM_COLON, EOF:
        left = nil
    case RParen, RightSBrace:
        panic(fmt.Errorf("Unqualified '%s' found", (*ct).tokText))

    case NumericLiteral:
        left = (*ct).tokVal
    case StringLiteral:
        left = interpolate(p.namespace, p.fs, p.ident, (*ct).tokText)
    case Identifier:
        left, err = p.identifier(ct)
        if err != nil {
            // fmt.Printf("Identifer case will panic with %#+v\n",err)
            panic(err)
        }
    case O_Sqr, O_Sqrt, O_InFile:
        left = p.unary(ct)
    case SYM_Caret: // range len
        if p.in_range {
            left = p.rangelen
        }
    case SYM_Not:
        right, err := p.dparse(24, false) // don't bind negate as tightly
        if err != nil {
            panic(err)
        }
        left = unaryNegate(right)
    case O_Pb, O_Pa, O_Pn, O_Pe, O_Pp:
        right, err := p.dparse(10, false) // allow strings to accumulate to the right
        if err != nil {
            panic(err)
        }
        left = p.unaryPathOp(right, (*ct).tokType)

    case SYM_DOT:

        if !(p.inside_with_struct || p.inside_with_enum) {
            panic("unary dot field operator present outside of a WITH clause.")
        } else {
            left = p.unary(ct)
        }

    case O_Slc, O_Suc, O_Sst, O_Slt, O_Srt:
        left = p.unary(ct)
    case O_Assign, O_Plus, O_Minus: // prec variable
        left = p.unary(ct)
    case LParen:
        left = p.grouping(ct)
    case SYM_PP, SYM_MM:
        left = p.preIncDec(ct)
    case LeftSBrace:
        left = p.array_concat(ct)
    case T_Map:
        // Save current position
        originalPos := p.pos
        result, handled := p.map_literal(ct)
        if handled {
            left = result
        } else {
            // Restore position and let normal parsing continue
            p.pos = originalPos
            // Continue with normal parsing (don't set left, let it be handled later)
        }
    case O_Ref:
        left = p.reference(false)
    case O_Mut:
        left = p.reference(true)
    case SYM_BOR:
        left = p.command()
    case Block: // ${
        _, left, _, _ = p.blockCommand(ct.tokText, false)
    case AsyncBlock: // &{
        _, _, _, left = p.blockCommand(ct.tokText, true)
    case ResultBlock: // {
        _, _, left, _ = p.blockCommand(ct.tokText, false)
    }

    var right any

    // binaries
binloop1:
    for {

        // pf("[cprec->%d tokprec->%d]\n",prec,p.prectable[p.peek().tokType])
        if prec >= p.prectable[p.peek().tokType] && p.pos < p.len && !p.namespacing {
            break
        }

        token := p.next()
        // pf("binloop nt -> %v at pos %d\n",token.tokText,p.pos)

        if p.namespacing {
            // pf("  (eval) namespacing, next token %v at %d\n",token.tokText,p.pos)
            if p.pos == p.namespace_pos+1 {
                p.namespacing = false
                left = p.prev2.tokText + "::" + token.tokText
                // pf("  (eval) completed namespace -> %#v at pos %d\n",left,p.pos)

                // Check if this is a module constant
                moduleConstantsLock.RLock()
                if constMap, exists := moduleConstants[p.prev2.tokText]; exists {
                    if val, found := constMap[token.tokText]; found {
                        moduleConstantsLock.RUnlock()
                        left = val
                        continue
                    }
                }
                moduleConstantsLock.RUnlock()

                continue
            }
        }

        switch token.tokType {
        case EOF:
            break binloop1

        case O_Query: // handle ? at end of expression
            if p.pos == p.len-1 {
                left = p.tern_if(nil, nil)
                continue
            }

        case O_Try: // handle ?? at end of expression
            if p.pos == p.len-1 {
                left = p.tryOperator(left, nil)
            } else {
                right, err = p.dparse(p.prectable[token.tokType]+1, false)
                if err == nil {
                    left = p.tryOperator(left, right)
                }
                panic(fmt.Errorf("Invalid expression in try operator message"))
            }
            continue

        case SYM_LAND:

            if !asBool(left) {
                // short-circuit: parse right just to consume tokens
                _, _ = p.dparse(p.prectable[token.tokType]+1, true)
                left = false
                continue
            }

            right, err = p.dparse(p.prectable[token.tokType]+1, false)
            if err != nil {
                panic(err)
            }
            left = asBool(right)
            continue

        case SYM_LOR, C_Or:

            if lstr, lok := left.(string); lok {
                if lstr != "" {
                    // left is non-empty → short-circuit: parse right just to consume tokens
                    _, _ = p.dparse(p.prectable[token.tokType]+1, true)
                    left = lstr
                    continue
                }

                // left is empty → parse right to get fallback
                right, err = p.dparse(p.prectable[token.tokType]+1, false)
                if err != nil {
                    panic(err)
                }

                if rstr, rok := right.(string); rok {
                    left = rstr
                } else {
                    left = right
                }
                continue
            }

            // fallback for booleans
            if asBool(left) {
                // short-circuit: left is true, must parse right to consume tokens
                _, _ = p.dparse(p.prectable[token.tokType]+1, true)
                left = true
                continue
            }

            // left is false: parse right normally
            right, err = p.dparse(p.prectable[token.tokType]+1, false)
            if err != nil {
                panic(err)
            }
            left = asBool(right)
            continue

        case SYM_PP, SYM_MM:
            left = p.postIncDec(token)
            continue
        case LeftSBrace:
            left = p.accessArray(left, token)
            continue
        case SYM_DoubleColon:
            if !p.namespacing {
                p.namespacing = true
                p.namespace_pos = p.pos
            } else {
                // pf(":: namespacing fault on token '%s' npos %d cpos %d?\nall toks -> %#v\n",token.tokText,p.namespace_pos,p.pos,p.tokens)
                p.namespacing = false
                p.namespace_pos = -1
                break binloop1
            }
            continue
        case SYM_DOT:
            p.std_faulted = false
            left, _ = p.accessFieldOrFunc(left, p.next().tokText)
            continue
        case C_Is:
            left = p.kind_compare(left)
            continue
        case LParen:
            switch left.(type) {
            case string:
                left, err = p.buildStructOrFunction(left, token)
                if err != nil {
                    return nil, err
                }
                continue
            }

        }

        if p.pos >= p.len {
            estring := "Incomplete expression, terminates early (on "
            if p.prev2.tokType < END_STATEMENTS {
                estring += sf("token-2 [%s], ", tokNames[p.prev2.tokType])
            }
            if p.prev.tokType < END_STATEMENTS {
                estring += sf("token-1 [%s], ", tokNames[p.prev.tokType])
            }
            if token.tokType < END_STATEMENTS {
                estring += sf("token [%s], ", tokNames[token.tokType])
            }
            estring += ")"
            panic(estring)
        }

        right, err = p.dparse(p.prectable[token.tokType]+1, false)

        switch token.tokType {

        case O_Plus:
            leftInt, lok := left.(int)
            rightInt, rok := right.(int)
            if lok && rok {
                left = leftInt + rightInt
            } else {
                left = ev_add(left, right)
            }
        case O_Minus:
            if isMap(left) && isMap(right) {
                left = differenceMaps(left.(map[string]any), right.(map[string]any))
            } else {
                left = ev_sub(left, right)
            }
        case O_Multiply:
            left = ev_mul(left, right)
        case O_Divide:
            left = ev_div(left, right)
        case O_Percent:
            left = ev_mod(left, right)

        case O_Query: // ternary
            left = p.tern_if(left, right)

        case SYM_EQ:
            left = deepEqual(left, right)
        case SYM_NE:
            left = !deepEqual(left, right)
        case SYM_LT:
            left = compare(left, right, SYM_LT)
        case SYM_GT:
            left = compare(left, right, SYM_GT)
        case SYM_LE:
            left = compare(left, right, SYM_LE)
        case SYM_GE:
            left = compare(left, right, SYM_GE)

        case SYM_Tilde:
            left = p.rcompare(left, right, false, false)
        case SYM_ITilde:
            left = p.rcompare(left, right, true, false)
        case SYM_FTilde:
            left = p.rcompare(left, right, false, true)

        case O_Filter:
            left = p.list_filter(left, right)
        case O_Map:
            left = p.list_map(left, right)

        case O_OutFile: // returns success/failure bool
            left = p.file_out(left, right)

        case SYM_BAND: // bitwise-and OR map intersection
            if isMap(left) && isMap(right) {
                left = intersectMaps(left.(map[string]any), right.(map[string]any))
            } else {
                left = as_integer(left) & as_integer(right)
            }
        case SYM_BOR: // bitwise-or OR map union
            if isMap(left) && isMap(right) {
                left = deepMergeMaps(left.(map[string]any), right.(map[string]any))
            } else {
                left = as_integer(left) | as_integer(right)
            }
        case SYM_LSHIFT:
            left = ev_shift_left(left, right)
        case SYM_RSHIFT:
            left = ev_shift_right(left, right)
        case SYM_Caret: // XOR OR map symmetric difference
            if isMap(left) && isMap(right) {
                left = symmetricDifferenceMaps(left.(map[string]any), right.(map[string]any))
            } else {
                left = as_integer(left) ^ as_integer(right)
            }
        case SYM_POW:
            left = ev_pow(left, right)
        case SYM_RANGE:
            left = ev_range(left, right)
        case C_In:
            left = ev_in(left, right)

        case O_Assign:
            panic(fmt.Errorf("assignment is not a valid operation in expressions"))

        default:
            panic(fmt.Errorf(" [ broken on type %s '%s'? ] ", tokNames[token.tokType], token.tokText))

        }

    }

    // pf("end-of-bin-loop\n")
    /*
       if err!=nil || left==nil {
         pf("[#2]dparse result: %+v[#-]\n",left)
         pf("[#2]dparse error : %#v[#-]\n",err)
       }
    */

    // Check for active exceptions before returning (lock-free)
    if exceptionPtr := atomic.LoadPointer(&calltable[p.fs].activeException); exceptionPtr != nil {
        // Only re-throw ?? operator exceptions, not normal function exceptions
        // Normal function exceptions should be handled by the main loop
        currentCatchMatched := atomic.LoadInt32(&calltable[p.fs].currentCatchMatched)

        if currentCatchMatched == 1 {
            // We're in a catch block - don't re-throw as it's already being handled
        } else {
            // Check if this is a ?? operator exception (source = "try_operator")
            excInfo := (*exceptionInfo)(exceptionPtr)
            if excInfo.source == "try_operator" {
                // This is a ?? operator exception - re-throw so it can be handled by try/catch or bubble up
                panic(ExceptionThrow{
                    Category: excInfo.category,
                    Message:  excInfo.message,
                })
            }
            // For normal function exceptions, let the main loop handle them
        }
    }

    return left, err
}

type rule struct {
    nud  func(token Token) any
    led  func(left any, token Token) any
    prec int8
}

func (p *leparser) ignore(token Token) any {
    p.next()
    return nil
}

// isContainerType checks if a value is a container type (slice, map, or struct)
func isContainerType(v any) bool {
    if v == nil {
        return false
    }

    t := reflect.TypeOf(v)
    switch t.Kind() {
    case reflect.Slice, reflect.Map, reflect.Struct:
        return true
    default:
        return false
    }
}

// processConditionString processes a condition string by applying all standard replacements
// This is the common helper function used by list_filter(), evaluateConditionForElements(), find(), where(), etc.
func processConditionString(condition string, element any, index int) (string, error) {
    // Start with the original expression
    newCondition := condition

    // Apply $idx replacement
    newCondition = str.Replace(newCondition, "$idx", strconv.Itoa(index), -1)

    // Handle #[index] patterns if element is a slice
    if str.Contains(newCondition, "#[") && isContainerType(element) {
        var err error
        newCondition, err = replaceAllArrayIndexing(newCondition, element)
        if err != nil {
            return "", err
        }
    }

    // Handle #.field access for structs and maps
    if str.Contains(newCondition, "#.") {
        elemValue := reflect.ValueOf(element)
        if elemValue.Kind() == reflect.Struct || elemValue.Kind() == reflect.Map {
            newCondition = replaceStructFieldAccess(newCondition, element)
        } else {
            // For non-structs, replace # with element literal
            newCondition = str.Replace(newCondition, "#", goLiteralToZaLiteral(element), -1)
        }
    } else {
        // Handle simple # replacement for backward compatibility
        newCondition = str.Replace(newCondition, "#", goLiteralToZaLiteral(element), -1)
    }

    return newCondition, nil
}

// replaceStructFieldAccess replaces #.field patterns with direct field access
func replaceStructFieldAccess(expr string, structVal any) string {
    result := expr

    for str.Contains(result, "#.") {
        hashDotPos := str.Index(result, "#.")
        if hashDotPos == -1 {
            break
        }

        // Find the end of the field name
        fieldStart := hashDotPos + 2
        fieldEnd := fieldStart
        for fieldEnd < len(result) {
            c := result[fieldEnd]
            if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
                break
            }
            fieldEnd++
        }

        if fieldEnd == fieldStart {
            // No field name found, skip
            result = result[:hashDotPos] + result[hashDotPos+2:]
            continue
        }

        fieldName := result[fieldStart:fieldEnd]

        // Use reflection to get the field value
        elemValue := reflect.ValueOf(structVal)
        var fieldValue string

        if elemValue.Kind() == reflect.Map {
            // Handle map access - maps store with lowercase keys
            mapValue := elemValue.MapIndex(reflect.ValueOf(fieldName))
            if mapValue.IsValid() {
                fieldValue = goLiteralToZaLiteral(mapValue.Interface())
            } else {
                fieldValue = "nil"
            }
        } else {
            // Handle struct field access - use renameSF() as specified in plan
            capitalizedFieldName := renameSF(fieldName)
            field := elemValue.FieldByName(capitalizedFieldName)
            if field.IsValid() {
                fieldValue = goLiteralToZaLiteral(field.Interface())
            } else {
                // Field doesn't exist, use nil
                fieldValue = "nil"
            }
        }

        // Replace the #.field pattern with the field value
        result = result[:hashDotPos] + fieldValue + result[fieldEnd:]
    }

    return result
}

// evaluateConditionForElements evaluates a condition string for each element in a slice
// Returns a slice of boolean results indicating which elements satisfy the condition
// This reuses the same logic as list_filter() but returns boolean results instead of filtered elements
func evaluateConditionForElements(elements []any, condition string, parser *leparser, fs uint32) ([]bool, error) {
    var reduceparser *leparser
    reduceparser = &leparser{}
    reduceparser.ident = parser.ident
    reduceparser.fs = fs
    reduceparser.ctx = parser.ctx

    results := make([]bool, len(elements))

    for i, element := range elements {
        // Use common condition string processing
        newCondition, err := processConditionString(condition, element, i)
        if err != nil {
            return nil, err
        }

        val, err := ev(reduceparser, fs, newCondition)
        if err != nil {
            return nil, err
        }

        switch val.(type) {
        case bool:
            results[i] = val.(bool)
        default:
            return nil, fmt.Errorf("invalid expression (non-boolean?) (%s) in condition evaluation", newCondition)
        }
    }

    return results, nil
}

func (p *leparser) list_filter(left any, right any) any {
    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("invalid condition string (%+v) in filter", right))
    }

    var reduceparser *leparser
    reduceparser = &leparser{}
    reduceparser.ident = p.ident
    reduceparser.fs = p.fs
    reduceparser.ctx = p.ctx

    // Check if left is a slice using reflection
    leftValue := reflect.ValueOf(left)
    leftType := reflect.TypeOf(left)

    // Handle slice filtering
    if leftType.Kind() == reflect.Slice {
        sliceLen := leftValue.Len()
        newSlice := reflect.MakeSlice(leftType, 0, sliceLen)

        for i := 0; i < sliceLen; i++ {
            element := leftValue.Index(i).Interface()

            // Use common condition string processing
            newRight, err := processConditionString(right.(string), element, i)
            if err != nil {
                panic(err)
            }

            val, err := ev(reduceparser, p.fs, newRight)
            if err != nil {
                panic(err)
            }

            switch val.(type) {
            case bool:
                if val.(bool) {
                    newSlice = reflect.Append(newSlice, leftValue.Index(i))
                }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter", newRight))
            }
        }

        return newSlice.Interface()
    }

    // Handle map filtering
    if leftType.Kind() == reflect.Map {
        newMap := reflect.MakeMap(leftType)
        iter := leftValue.MapRange()

        for iter.Next() {
            key := iter.Key()
            valueRef := iter.Value()
            value := valueRef.Interface()

            // Use common condition string processing (note: maps don't have index, use -1)
            newRight, err := processConditionString(right.(string), value, -1)
            if err != nil {
                panic(err)
            }

            val, err := ev(reduceparser, p.fs, newRight)
            if err != nil {
                panic(err)
            }

            switch val.(type) {
            case bool:
                if val.(bool) {
                    newMap.SetMapIndex(key, valueRef)
                }
            default:
                panic(fmt.Errorf("invalid expression (non-boolean?) (%s) in filter", newRight))
            }
        }

        return newMap.Interface()
    }

    panic(fmt.Errorf("filter: unsupported left operand type: %T", left))
}

func (p *leparser) file_out(left any, right any) any {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("$out requires a filename string on right-hand side"))
    }

    switch left.(type) {
    case string:
    default:
        panic(fmt.Errorf("$out requires an output string on left-hand side"))
    }

    err := ioutil.WriteFile(right.(string), []byte(left.(string)), 0600)
    if err != nil {
        return false
    }
    return true

}

func convertToElementType(val any, elemType reflect.Type) any {
    valValue := reflect.ValueOf(val)

    // If types already match, return as-is
    if valValue.Type().AssignableTo(elemType) {
        return val
    }

    // Handle conversions to *big.Int
    if elemType == reflect.TypeOf(&big.Int{}) {
        switch v := val.(type) {
        case int:
            return big.NewInt(int64(v))
        case int64:
            return big.NewInt(v)
        case float64:
            return big.NewInt(int64(v))
        case *big.Int:
            return v
        }
    }

    // Handle conversions to *big.Float
    if elemType == reflect.TypeOf(&big.Float{}) {
        switch v := val.(type) {
        case int:
            return big.NewFloat(float64(v))
        case int64:
            return big.NewFloat(float64(v))
        case float64:
            return big.NewFloat(v)
        case *big.Float:
            return v
        }
    }

    // Handle basic numeric conversions
    switch elemType.Kind() {
    case reflect.Int:
        switch v := val.(type) {
        case int:
            return v
        case int64:
            return int(v)
        case float64:
            return int(v)
        }
    case reflect.Float64:
        switch v := val.(type) {
        case int:
            return float64(v)
        case int64:
            return float64(v)
        case float64:
            return v
        }
    case reflect.String:
        return goLiteralToZaLiteral(val)
    }

    // If no conversion needed or possible, return as-is
    return val
}

func (p *leparser) list_map(left any, right any) any {

    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("invalid string (%+v) in map", right))
    }

    var reduceparser *leparser
    reduceparser = &leparser{}
    reduceparser.ident = p.ident
    reduceparser.fs = p.fs
    reduceparser.ctx = p.ctx

    // Check if left is a slice using reflection
    leftValue := reflect.ValueOf(left)
    leftType := reflect.TypeOf(left)

    // Handle strings by converting to character slices
    if leftType.Kind() == reflect.String {
        str := left.(string)
        // Convert each character to a string element
        charSlice := make([]any, len(str))
        for i, char := range str {
            charSlice[i] = string(char)
        }
        left = charSlice
        leftValue = reflect.ValueOf(left)
        leftType = reflect.TypeOf(left)
    }

    // Handle slice mapping
    if leftType.Kind() == reflect.Slice {
        sliceLen := leftValue.Len()

        // Handle empty slice case
        if sliceLen == 0 {
            return []any{}
        }

        // Determine result type by evaluating first element
        firstElement := leftValue.Index(0).Interface()

        // Use common condition string processing for first element
        newRight, err := processConditionString(right.(string), firstElement, 0)
        if err != nil {
            panic(err)
        }

        firstResult, err := ev(reduceparser, p.fs, newRight)
        if err != nil {
            panic(err)
        }

        // Create slice of result type
        resultType := reflect.TypeOf(firstResult)
        newSlice := reflect.MakeSlice(reflect.SliceOf(resultType), 0, sliceLen)
        newSlice = reflect.Append(newSlice, reflect.ValueOf(firstResult))

        // Process remaining elements
        for i := 1; i < sliceLen; i++ {
            element := leftValue.Index(i).Interface()

            // Use common condition string processing
            newRight, err := processConditionString(right.(string), element, i)
            if err != nil {
                panic(err)
            }

            val, err := ev(reduceparser, p.fs, newRight)
            if err != nil {
                panic(err)
            }

            // Convert val to match the result type
            val = convertToElementType(val, resultType)

            // For map operations, we append the result value (not boolean filtering)
            newSlice = reflect.Append(newSlice, reflect.ValueOf(val))
        }

        return newSlice.Interface()
    }

    // Handle map mapping
    if leftType.Kind() == reflect.Map {
        keyType := leftType.Key()
        valueType := leftType.Elem()
        newMap := reflect.MakeMap(reflect.MapOf(keyType, valueType))

        // Convert map to slice of key-value pairs to avoid iterator issues
        mapKeys := leftValue.MapKeys()
        for i := 0; i < len(mapKeys); i++ {
            key := mapKeys[i]
            valueRef := leftValue.MapIndex(key)
            value := valueRef.Interface()

            // Use common condition string processing (note: maps don't have index, use -1)
            newRight, err := processConditionString(right.(string), value, -1)
            if err != nil {
                panic(err)
            }

            val, err := ev(reduceparser, p.fs, newRight)
            if err != nil {
                panic(err)
            }

            // Set result in map
            newMap.SetMapIndex(key, reflect.ValueOf(val))
        }

        return newMap.Interface()
    }

    panic(fmt.Errorf("map: unsupported left operand type: %T", left))
}

func (p *leparser) rcompare(left any, right any, insensitive bool, multi bool) any {
    switch left.(type) {
    case string:
    default:
        panic(fmt.Errorf("regex comparison requires strings"))
    }
    switch right.(type) {
    case string:
    default:
        panic(fmt.Errorf("regex comparison requires strings"))
    }

    insenStr := ""
    if insensitive {
        insenStr = "(?i)"
    }
    key := insenStr + right.(string)

    // Attempt to load from sync.Map
    v, ok := ifCompileCache.Load(key)
    var re *regexp.Regexp
    if ok {
        re = v.(*regexp.Regexp)
    } else {
        // Compile and store
        compiled, err := regexp.Compile(key)
        if err != nil {
            panic(fmt.Errorf("supplied regex is invalid: %s", right.(string)))
        }
        ifCompileCache.Store(key, compiled)
        re = compiled
    }

    if multi {
        return re.FindAllString(left.(string), -1)
    }
    return re.MatchString(left.(string))
}

func (p *leparser) accessArray(left any, right Token) any {

    // pf("p.aa inbound left array is : %+v\n",left)

    var start, end any
    var hasStart, hasEnd, hasRange bool
    var sendNil bool

    if !p.in_range {
        p.in_range = true
        defer func() { p.in_range = false }()
    }

    switch left := left.(type) {

    case []tui:
        p.rangelen = len(left)
    case []bool:
        p.rangelen = len(left)
    case []string:
        p.rangelen = len(left)
    case []int:
        p.rangelen = len(left)
    case []uint:
        p.rangelen = len(left)
    case []float64:
        p.rangelen = len(left)
    case []dirent:
        p.rangelen = len(left)
    case []alloc_info:
        p.rangelen = len(left)
    case []stackFrame:
        p.rangelen = len(left)
    case string:
        p.rangelen = len(left)
    case []*big.Int:
        p.rangelen = len(left)
    case []*big.Float:
        p.rangelen = len(left)
    case [][]int:
        p.rangelen = len(left)
    case []any:
        p.rangelen = len(left)

    case map[string]any, map[string][]any, map[string]alloc_info, map[string]tui, map[string]string, map[string]int:

        // check for key
        var mkey string
        if right.tokType == SYM_DOT {
            t := p.next()
            mkey = t.tokText
        } else {
            if p.peek().tokType != RightSBrace {
                dp, err := p.dparse(0, false)
                if err != nil {
                    panic(fmt.Errorf("map key could not be evaluated"))
                    return nil
                }
                switch dp.(type) {
                case string:
                    mkey = dp.(string)
                default:
                    mkey = sf("%v", dp)
                }
            }
            if p.peek().tokType != RightSBrace {
                panic(fmt.Errorf("end of map key brace missing"))
            }
            // swallow right brace
            p.next()
        }
        return accessArray(p.ident, left, mkey)

        // end map case

    case uint, int, float64, uint8, uint64, int64, *big.Int, *big.Float:
        // just allow these through. handled as a clamp operation later.
        // but do flag to allow missing start/end
        hasRange = true
    default:
        // Handle kdynamic types using reflection
        rval := reflect.ValueOf(left)
        if rval.Kind() == reflect.Slice || rval.Kind() == reflect.Array {
            p.rangelen = rval.Len()
        } else {
            sendNil = true
        }
    }

    if p.peek().tokType != RightSBrace {

        // check for start of range
        if p.peek().tokType != SYM_COLON {
            // pf("(aa)     ntok -> %+v\n",tokNames[p.peek().tokType])
            dp, err := p.dparse(0, false)
            // pf("(aa) start dp -> %+v\n",dp)
            // pf("(aa)   err dp -> %+v\n",err)
            if err != nil {
                panic(fmt.Errorf("array range start could not be evaluated"))
            }
            switch dp.(type) {
            case int, float64, *big.Int, *big.Float:
                start = dp
                hasStart = true
            }
        }

        // check for end of range
        if p.peek().tokType == SYM_COLON {
            p.next() // swallow colon
            hasRange = true
            if p.peek().tokType != RightSBrace {
                dp, err := p.dparse(0, false)
                if err != nil {
                    panic(fmt.Errorf("array range end could not be evaluated"))
                }
                switch dp.(type) {
                case int, float64, *big.Int, *big.Float:
                    end = dp
                    hasEnd = true
                }
            }
        }

        // pf("[range] next token %v\n",tokNames[p.peek().tokType])
        if p.peek().tokType != RightSBrace {
            panic(fmt.Errorf("end of range brace missing"))
        }

        // swallow brace
        p.next()

    }

    if sendNil {
        return nil
    }

    if !hasRange && !hasStart && !hasEnd {
        hasRange = true
    }

    switch hasRange {
    case false:
        return accessArray(p.ident, left, start)
    case true:
        return slice(left, start, end)
    }

    return nil

}

func (p *leparser) buildStructOrFunction(left any, right Token) (any, error) {

    name := left.(string)
    isStruct := false

    // Special handling for C library namespaces: check if it's a C function first
    // This prevents struct constructors from shadowing C functions (e.g., c::stat)
    isCLibFunction := false
    if str.Contains(name, "::") {
        parts := str.SplitN(name, "::", 2)
        if len(parts) == 2 && isCFunction(parts[0], parts[1]) {
            isCLibFunction = true
        }
    }

    // filter for enabling struct type names here:
    // But skip struct check if this is a C library function
    structvalues := []any{}
    found := false
    if !isCLibFunction {
        // Resolve struct name through use_chain
        resolvedName := uc_match_struct(name)
        lookupName := name
        if resolvedName != "" {
            lookupName = resolvedName + "::" + name
        }

        structmapslock.RLock()
        if structvalues, found = structmaps[lookupName]; !found {
            // Fallback to exact lookup
            structvalues, found = structmaps[name]
        }
        if found || name == "anon" {
            isStruct = true
        }
        structmapslock.RUnlock()
    }
    // end-struct-filter

    if !isStruct {
        // filter for functions here
        var isFunc bool

        // check if exists in user defined function space
        if _, isFunc = stdlib[name]; !isFunc {
            if !str.Contains(name, "::") {
                var useName string
                if found := uc_match_func(name); found != "" {
                    useName = found + "::" + name
                } else {
                    useName = "main::" + name
                    if len(p.namespace) > 0 {
                        useName = p.namespace + "::" + name
                    }
                }
                name = useName
            }
            isFunc = fnlookup.lmexists(name)

            // Check for C functions as fallback (before panic)
            if !isFunc {
                if namespace := FindCFunction(name); namespace != "" {
                    isFunc = true
                }
            }
        }

        if !isFunc {
            panic(fmt.Errorf("'%v' is not a function", name))
        }
    }

    iargs := []any{}
    arg_names := []string{}
    argpos := 1
    if p.peek().tokType != RParen {
        for {
            switch p.peek().tokType {
            case SYM_DOT:
                p.next()                                               // move-to-dot
                p.next()                                               // skip-to-name-from-dot
                arg_names = append(arg_names, p.tokens[p.pos].tokText) // add name field
            case RParen, O_Comma:
                // missing/blank arg in list
                panic(fmt.Errorf("missing argument #%d", argpos))
            }
            dp, err := p.dparse(0, false)
            if err != nil {
                panic(fmt.Errorf("error here -> %+v\n", err))
                return nil, err
            }
            iargs = append(iargs, dp)
            if p.peek().tokType != O_Comma {
                break
            }
            p.next()
            argpos += 1
        }
    }

    if p.peek().tokType == RParen {
        p.next() // consume rparen
    }

    // build struct literals
    if isStruct {

        var t Variable

        if len(arg_names) > 0 {

            // enforce initial case
            for i, an := range arg_names {
                arg_names[i] = renameSF(an)
            }

            // named field handling:
            //  struct_name(.name value,...,.name value)
            if len(arg_names) == len(iargs) {
                // all dotted, named fields?
                /*
                   for n:=0; n<len(arg_names); n+=1 {
                       pf("s-field, loop name  #%d : %+v\n",n,arg_names[n])
                       pf("s-field, loop value #%d : %+v\n",n,iargs[n])
                   }
                */
            } else {
                panic(fmt.Errorf("length mismatch of argument names [%d] to struct fields [%d]", len(arg_names), len(iargs)))
            }
        }

        if name == "anon" {
            for n := 0; n < len(arg_names); n += 1 {
                structvalues = append(structvalues, arg_names[n])
                t := reflect.TypeOf(iargs[n])
                typeFound := false
                for vk, vt := range Typemap {
                    if vt == t {
                        structvalues = append(structvalues, vk)
                        typeFound = true
                        break
                    }
                }
                if !typeFound {
                    panic(fmt.Errorf("unknown type in struct(anon) field %s [%v]", arg_names[n], t))
                }
                structvalues = append(structvalues, true)
                structvalues = append(structvalues, iargs[n])
            }
        } else {
            switch len(iargs) {
            case 0:
                // leave 0 args as unhandle, for a default constructor here
            case len(structvalues) / 4:
                // work through iargs, populating struct fields here
                // structvalues: [0] name [1] type [2] boolhasdefault [3] default_value

                // confirm types match named arguments:
                if len(arg_names) > 0 {
                    for i := range iargs {
                        nameMatched := false
                        for j := 0; j < len(structvalues); j += 4 {
                            if structvalues[j].(string) == arg_names[i] {
                                fieldType := structvalues[j+1].(string)
                                // Skip type check for "any" and "mixed" types - they accept any value
                                if fieldType != "any" && fieldType != "mixed" {
                                    if Typemap[fieldType] != reflect.TypeOf(iargs[i]) {
                                        panic(fmt.Errorf("type mismatch in named field '%s', should be %v", arg_names[i], structvalues[j+1]))
                                    }
                                }
                                nameMatched = true
                                break // found a positive match, move on to next argument
                            }
                        }
                        if !nameMatched {
                            panic(fmt.Errorf("provided argument name '%s' not found in struct '%s'", arg_names[i], name))
                        }
                    }
                    // if we reach here, then all types matched the provided values, hopefully!
                }

                n := 0
                for i := 3; i < len(structvalues); i += 4 {
                    structvalues[i-1] = true
                    structvalues[i] = iargs[n]
                    n += 1
                }
            default:
                // error
                panic(fmt.Errorf("invalid parameter list count (%d) in struct(%s) init", len(iargs), name))
            }
        }

        err := fillStruct(&t, structvalues, Typemap, false, arg_names)
        if err != nil {
            panic(err)
        }

        return t.IValue, nil

    }

    // if not a struct() then treat as a normal func() instead:

    if len(arg_names) > 0 { // check that arg_names tally with functionArgs list
        var ifn uint32
        var present bool
        if ifn, present = fnlookup.lmget(name); !present {
            panic(fmt.Errorf("could not find function named '%s'", name))
        }
        farglock.RLock()
        falist := functionArgs[ifn].args
        farglock.RUnlock()
        if len(arg_names) == len(falist) {
            for _, an := range arg_names {
                found := false
                for _, fa := range falist {
                    if an == fa {
                        found = true
                        break
                    }
                }
                if !found {
                    panic(fmt.Errorf("argument '%s' not found in definition for '%s'", an, name))
                }
            }
        } else {
            panic(fmt.Errorf("bad argument name count [%d] for '%s' [needs %d]", len(arg_names), name, len(falist)))
        }
    }

    // pf("entering cfe with %s args:%#v arg_names:%#v\n",name,iargs,arg_names)
    res, _, _, err := p.callFunctionExt(p.fs, p.ident, name, false, nil, "", arg_names, iargs)
    if err != nil {
        panic(fmt.Errorf("%+v\n", err))
        return nil, err
    }

    // fmt.Printf("cleanly exiting from buildStructOrFunction() with result : %#v\n",res)

    return res, err

}

// reference returns identifier name for ref, or MutableArg wrapper for mut
func (p *leparser) reference(mut bool) any {
    vartok := p.next()

    // Check if next token is :: (namespace operator)
    fullName := vartok.tokText
    if p.peek().tokType == SYM_DoubleColon {
        p.next() // consume ::
        identTok := p.next() // consume the identifier after ::
        fullName = vartok.tokText + "::" + identTok.tokText
    }

    // Get the correct binding index for the full name
    bin := bind_int(p.fs, fullName)

    if mut {
        // First check if variable exists in local scope
        if bin < uint64(len(*p.ident)) && (*p.ident)[bin].declared {
            // Get variable value and wrap it
            varValue := (*p.ident)[bin].IValue

            return &MutableArg{
                Value:    varValue,
                Binding:  bin,
                IdentPtr: p.ident,
                IsGlobal: false,
                // CPtr and StructDef will be set by FFI layer during marshaling
            }
        }

        // Variable not found in local scope, check globals
        gbin := bind_int(0, fullName)
        if gbin < uint64(len(gident)) && gident[gbin].declared {
            // Get global variable value with proper locking
            varValue, ok := gvget(fullName)
            if ok {
                return &MutableArg{
                    Value:    varValue,
                    Binding:  gbin,
                    IdentPtr: &gident,
                    IsGlobal: true,
                    // CPtr and StructDef will be set by FFI layer during marshaling
                }
            }
        }

        // Variable not declared in either local or global scope - will error later in identifier()
        return nil
    }

    // Original ref behavior - just return identifier name
    return fullName
}

func (p *leparser) unaryPathOp(right any, op int64) string {
    switch right.(type) {
    case string:
        switch op {
        case O_Pb: // base path
            return filepath.Base(right.(string))
        case O_Pa: // abs path
            fp, e := filepath.Abs(right.(string))
            if e != nil {
                return ""
            }
            return fp
        case O_Pn: // base - no ext
            fp := filepath.Base(right.(string))
            fe := filepath.Ext(fp)
            if fe == "" {
                return fp
            }
            return fp[:len(fp)-len(fe)]
        case O_Pe: // base - only ext
            fpe := filepath.Ext(right.(string))
            if len(fpe) > 1 {
                return fpe[1:]
            }
            return ""
        case O_Pp: // parent path
            fp, e := filepath.Abs(right.(string))
            if e != nil {
                return ""
            }
            return fp[:str.LastIndex(fp, "/")]
        default:
            panic(fmt.Errorf("unknown unary path operator!")) // shouldn't see this!
        }
    default:
        panic(fmt.Errorf("invalid type in unary path operator"))
    }
}

// none of this pointer stuff is live. just tinkering here. move along!
func (p *leparser) unaryPointerOp(right any, op int64) any {
    bin := bind_int(p.fs, right.(string))
    switch op {
    case SYM_Caret:
        switch right.(type) {
        case string:
            if (*p.ident)[bin].declared {
                return &((*p.ident)[bin])
            }
        }
    case O_Multiply:
        return (*right.(*Variable)).IValue
    }
    return nil
}

func (p *leparser) unaryStringOp(right any, op int64) string {
    switch right.(type) {
    case string:
        switch op {
        case O_Slc:
            return str.ToLower(right.(string))
        case O_Suc:
            return str.ToUpper(right.(string))
        case O_Sst:
            return str.Trim(right.(string), " \t\n\r")
        case O_Slt:
            return str.TrimLeft(right.(string), " \t\n\r")
        case O_Srt:
            return str.TrimRight(right.(string), " \t\n\r")
        default:
            panic(fmt.Errorf("unknown unary string operator!"))
        }
    default:
        panic(fmt.Errorf("invalid type in unary string operator"))
    }
}

func (p *leparser) unary(token *Token) any {

    switch token.tokType {
    case O_InFile:
        right, err := p.dparse(70, false) // higher than dot op
        if err != nil {
            panic(err)
        }
        return unaryFileInput(right)
    case SYM_DOT:
        // get next token's tokText
        next_tok := p.peek()
        if next_tok.tokType == Identifier {
            p.next()
        } else {
            panic(fmt.Errorf("unary value is not an identifier [%s]", next_tok.tokText))
        }
        if p.inside_with_struct {
            var err bool
            bin := bind_int(p.fs, p.with_struct_name)
            tok := Token{tokType: Identifier, tokText: p.with_struct_name, bindpos: bin}
            left, _ := p.identifier(&tok)
            // pf("with_struct : left -> [%#v]\n",left)
            p.prev2 = tok
            p.std_faulted = false
            left, err = p.accessFieldOrFunc(left, next_tok.tokText)
            if left == nil {
                panic(fmt.Errorf("unary value is not a valid struct field [%s,with err:%v]", next_tok.tokText, err))
            }
            return left
        }
        if p.inside_with_enum {
            bin := bind_int(p.fs, p.with_enum_name)
            tok := Token{tokType: Identifier, tokText: p.with_enum_name, bindpos: bin}
            left, _ := p.identifier(&tok)
            p.prev2 = tok
            p.std_faulted = false
            left, _ = p.accessFieldOrFunc(left, next_tok.tokText)
            if left == nil {
                panic(fmt.Errorf("unary value is not a valid enum member [%s]", next_tok.tokText))
            }
            return left
        }
    }

    right, err := p.dparse(38, false) // between grouping and other ops
    if err != nil {
        panic(err)
    }

    switch token.tokType {
    case O_Minus:
        return unaryMinus(right)
    case O_Plus:
        return unaryPlus(right)
    case O_Sqr:
        return unOpSqr(right)
    case O_Sqrt:
        return unOpSqrt(right)
    case O_Slc, O_Suc, O_Sst, O_Slt, O_Srt:
        return p.unaryStringOp(right, token.tokType)
    case O_Assign:
        panic(fmt.Errorf("unary assignment makes no sense"))
    }

    return nil
}

func unOpSqr(n any) any {
    switch n := n.(type) {
    case int:
        return n * n
    case uint:
        return n * n
    case float64:
        return n * n
    case *big.Int:
        var tmp big.Int
        tmp.Set(n)
        return tmp.Mul(&tmp, n)
    case *big.Float:
        var tmp big.Float
        tmp.Set(n)
        return tmp.Mul(&tmp, n)
    default:
        panic(fmt.Errorf("sqr does not support type '%T'", n))
    }
}

func unOpSqrt(n any) any {
    switch n := n.(type) {
    case int:
        return math.Sqrt(float64(n))
    case uint:
        return math.Sqrt(float64(n))
    case float64:
        return math.Sqrt(n)
    case *big.Int:
        var tmp big.Int
        return tmp.Sqrt(n)
    case *big.Float:
        var tmp big.Float
        return tmp.Sqrt(n)
    default:
        panic(fmt.Errorf("sqrt does not support type '%T'", n))
    }
    // unreachable: // return nil
}

func (p *leparser) tern_if(left any, tv any) any {
    // expr '?' tv ':' fv
    switch left.(type) {
    case bool:
    default:
        panic(fmt.Errorf("not a boolean on left of ternary"))
    }
    if p.peek().tokType == SYM_COLON {
        p.next()
    } else {
        panic(fmt.Errorf("missing colon in ternary"))
    }

    switch left.(type) {
    case bool:
        if left.(bool) {
            p.dparse(0, true)
            return tv
        }
    }
    fv, err := p.dparse(0, false)
    if err != nil {
        panic(fmt.Errorf("malformed false expression in ternary"))
    }
    return fv
}

func (p *leparser) array_concat(tok *Token) any {

    // right-associative

    ary := []any{}

    if p.peek().tokType != RightSBrace {
        for {
            // bodge to allow trailing commas in array
            if p.peek().tokType == RightSBrace {
                break
            }
            // parse next expression in array
            dp, err := p.dparse(0, false)
            if err != nil {
                panic(err)
            }
            // add to array
            ary = append(ary, dp)
            // exit loop if no comma next
            if p.peek().tokType != O_Comma {
                break
            }
            // consume comma
            p.next()
        }
    }

    // consume rparen
    if p.peek().tokType == RightSBrace {
        // pf("[[trailing right-square-brace consumption]]\n")
        p.next()
    }

    // pf("[[array loop, trailing peek is '%s']]\n",tokNames[p.peek().tokType])
    return ary

}

func (p *leparser) map_literal(tok *Token) (any, bool) {
    // pf("(map_literal) called with token : %#v\n",tok)

    // Check if next token is LParen (function call syntax)
    if p.peek().tokType != LParen {
        // Not a map literal, signal not handled
        return nil, false
    }

    // Consume the left parenthesis
    p.next()

    // Parse arguments using the same logic as buildStructOrFunction
    iargs := []any{}
    arg_names := []string{}
    argpos := 1

    if p.peek().tokType != RParen {
        for {
            switch p.peek().tokType {
            case SYM_DOT:
                p.next() // consume dot
                key := ""
                // pf("sym_dot pre loop, pos->%d len->%d\n",p.pos,p.len)
                for {
                    // pf("nt->%+v pos->%d ckey->%s\n",tokNames[p.peek().tokType],p.pos,key)
                    if p.pos >= p.len {
                        pf("broke on len\n")
                        break
                    }
                    key += p.peek().tokText
                    p.next() // consume identifier/keyword

                    if p.peek().tokType == SYM_COLON {
                        // allow an optional colon, to avoid the
                        // negative number value issue below
                        p.next()
                        break
                    }
                    if p.pos >= p.len || p.peek().tokType != O_Minus {
                        // pf("broke on not minus/len\n")
                        break
                    }
                    if p.peek().tokType == O_Minus {
                        if p.tokens[p.pos+2].tokType == NumericLiteral {
                            // force a break here, otherwise negative numbers
                            // in the value will not be recognised properly
                            break
                        }
                    }

                    key += "-"
                    p.next() // consume -
                }
                arg_names = append(arg_names, key) // add name field
                // pf("added key '%s' to arg_names\n",key)
            case RParen, O_Comma:
                // missing/blank arg in list
                panic(fmt.Errorf("missing argument #%d", argpos))
            }
            // pf("p.pos->%d peek()->%+v\n",p.pos,p.peek())
            dp, err := p.dparse(0, false)
            if err != nil {
                panic(fmt.Errorf("error parsing map argument -> %+v\n", err))
            }
            iargs = append(iargs, dp)
            if p.peek().tokType != O_Comma {
                // pf("broke on not comma, was [%s]\n",tokNames[p.peek().tokType])
                break
            }
            p.next()
            argpos += 1
        }
    }

    if p.peek().tokType == RParen {
        p.next() // consume rparen
    } else {
        panic(fmt.Errorf("expected closing parenthesis for map literal, not [%s]", tokNames[p.peek().tokType]))
    }

    // Build the map from the parsed arguments
    result := make(map[string]any)

    if len(arg_names) == len(iargs) {
        // All arguments are named (using .name syntax)
        for i := 0; i < len(arg_names); i++ {
            result[arg_names[i]] = iargs[i]
        }
    } else {
        // Error: mismatched argument names and values
        panic(fmt.Errorf("length mismatch of argument names [%d] to values [%d]", len(arg_names), len(iargs)))
    }

    // pf("result=%#v\np.pos->%d\n",result,p.pos)
    return result, true
}

func (p *leparser) preIncDec(token *Token) any {

    // get direction
    ampl := 1
    switch token.tokType {
    case SYM_MM:
        ampl = -1
    }

    // move parser position to varname
    vartok := p.next()

    // exists?
    var val any

    bin := vartok.bindpos

    activeFS := p.fs
    if !(*p.ident)[bin].declared {
        gbin := bind_int(p.mident, vartok.tokText)
        if mident[gbin].declared {
            val, _ = vget(nil, p.mident, &mident, vartok.tokText)
            activeFS = p.mident
        } else {
            panic(fmt.Errorf("invalid variable name in pre-inc/dec '%s'", vartok.tokText))
        }
    } else {
        val, _ = vget(&vartok, p.fs, p.ident, vartok.tokText)
    }

    // act according to var type
    var n any
    switch v := val.(type) {
    case int:
        n = v + ampl
    case uint:
        n = v + uint(ampl)
    case float64:
        n = v + float64(ampl)
    case *big.Int:
        n = v.Add(v, GetAsBigInt(ampl))
    case *big.Float:
        n = v.Add(v, GetAsBigFloat(ampl))
    default:
        p.report(-1, sf("pre-inc/dec not supported on type '%T' (%s)", val, val))
        finish(false, ERR_EVAL)
        return nil
    }
    if activeFS == p.mident {
        vset(&vartok, p.mident, &mident, vartok.tokText, n)
    } else {
        vset(&vartok, p.fs, p.ident, vartok.tokText, n)
    }
    return n

}

func (p *leparser) postIncDec(token Token) any {

    // get direction
    ampl := 1
    switch token.tokType {
    case SYM_MM:
        ampl = -1
    }

    // get var from parser context
    vartok := p.prev

    // exists?
    var val any

    bin := vartok.bindpos
    activeFS := p.fs

    var mloc uint32
    if interactive {
        mloc = 1
    } else {
        mloc = 2
    }

    activePtr := p.ident

    if strcmp((*p.ident)[bin].IName, vartok.tokText) {
        if !(*p.ident)[bin].declared {
            gbin := bind_int(mloc, vartok.tokText)
            if mident[gbin].declared {
                val, _ = vget(&token, mloc, &mident, vartok.tokText)
                activeFS = mloc
                activePtr = &mident
            } else {
                panic(fmt.Errorf("invalid variable name in post-inc/dec '%s'", vartok.tokText))
            }
        } else {
            val, _ = vget(&vartok, p.fs, p.ident, vartok.tokText)
        }
    } else {
        panic(fmt.Errorf("'%s' not a local variable.", vartok.tokText))
    }

    // act according to var type
    switch v := val.(type) {
    case int:
        vset(&vartok, activeFS, activePtr, vartok.tokText, v+ampl)
    case uint:
        vset(&vartok, activeFS, activePtr, vartok.tokText, v+uint(ampl))
    case float64:
        vset(&vartok, activeFS, activePtr, vartok.tokText, v+float64(ampl))
    case *big.Int:
        n := v.Add(v, GetAsBigInt(ampl))
        vset(&vartok, activeFS, activePtr, vartok.tokText, n)
    case *big.Float:
        n := v.Add(v, GetAsBigFloat(ampl))
        vset(&vartok, activeFS, activePtr, vartok.tokText, n)
    default:
        panic(fmt.Errorf("post-inc/dec not supported on type '%T' (%s)", val, val))
    }
    return val
}

func (p *leparser) grouping(tok *Token) any {

    // right-associative
    val, err := p.dparse(0, false)
    if err != nil {
        panic(err)
    }
    p.next() // consume RParen
    return val

}

func (p *leparser) kind_compare(left any) bool {
    typeTok := p.next()
    return ev_kind_compare(left, typeTok)
}

func (p *leparser) number(token Token) (num any) {
    var err error

    // test code:
    num = token.tokVal

    if num == nil {
        panic(err)
    }
    return num
}

type cmd_result struct {
    Out  string
    Err  string
    Code int
    Okay bool
}
type bg_result struct {
    Name   string
    Handle chan any
}

func (p *leparser) blockCommand(cmd string, async bool) (state bool, resstr string, result cmd_result, bgresult bg_result) {

    cmd = sparkle(interpolate(p.namespace, p.fs, p.ident, cmd))

    if async {

        // make a new fn name
        csumName := sf("_bg_block_%x", md5.Sum([]byte(cmd)))

        // define fn
        stdlib["exec"](p.namespace, p.fs, p.ident, "define "+csumName+"()\nr={"+cmd+"\n}\nreturn r;end\n")

        // exec it async
        name := "main::" + csumName
        if len(p.namespace) != 0 {
            name = p.namespace + "::" + csumName
        }

        lmv, isfunc := fnlookup.lmget(name)

        if isfunc {
            // Register new function space with caller's filename
            // Use the source base of p.fs, not p.fs directly
            calllock.RLock()
            sourceBase := calltable[p.fs].base
            // We need to register the base that Call() will actually look up
            targetBase := calltable[lmv].base
            calllock.RUnlock()
            if callerFile, exists := fileMap.Load(sourceBase); exists {
                fileMap.Store(targetBase, callerFile)
            }

            // call
            h, id := task(p.fs, lmv, false, csumName+"@", nil)
            // destroy fn def before leaving
            fnlookup.lmdelete(p.namespace + "::" + csumName)
            numlookup.lmdelete(lmv)
            // return
            return true, "", cmd_result{}, bg_result{Name: id, Handle: h}
        }

        pf("Background process could not be generated.\n")
        return false, "", cmd_result{}, bg_result{}

    }

    result = system(cmd, false)
    return result.Okay, result.Out, result, bg_result{}

}

func (p *leparser) command() string {

    dp, err := p.dparse(65, false)
    if err != nil {
        panic(fmt.Errorf("error parsing string in command operator"))
    }

    switch dp.(type) {
    case string:
    default:
        panic(fmt.Errorf("command operator only accepts strings (not %T)", dp))
    }

    // pf("command : |%s|\n",dp.(string))
    cmd := system(interpolate(p.namespace, p.fs, p.ident, dp.(string)), false)

    if cmd.Okay {
        return cmd.Out
    }

    panic(fmt.Errorf("error in command operator (code:%d) '%s'", cmd.Code, cmd.Err))

}

func (p *leparser) identifier(token *Token) (any, error) {

    // pf("(identifier) got token -> %#v)\n", token)

    /*
       if token.tokType != Identifier {
           // fmt.Printf("error in identifier name: existing token type %s", tokNames[token.tokType])
           return nil, fmt.Errorf("error in identifier name: existing token type %s", tokNames[token.tokType])
       }
    */

    switch token.subtype {
    case subtypeConst:
        return token.tokVal, nil
    case subtypeStandard:
        return token.tokText, nil
    case subtypeUser,subtypeCUser:
        return token.tokText, nil
    }
    // pf("(identifier) reached past subtype check.\n")

    // filter for functions here. this also sets the subtype for funcs defined late.
    if p.pos+1 != p.len && p.tokens[p.pos+1].tokType == LParen {
        if _, isFunc := stdlib[token.tokText]; !isFunc {
            // pf("(identifier) inside isFunc? check. not a standard library function '%s'\n", token.tokText)
            var useName string
            if p.prev.tokType != SYM_DoubleColon {
                if found := uc_match_func(token.tokText); found != "" {
                    useName = found
                } else {
                    if len(p.namespace) > 0 {
                        useName = p.namespace
                    } else {
                        useName = "main"
                    }
                }
            }
            // pf("  -- checking for name %s::%s in:\n%#v\n", useName, token.tokText, fnlookup.lmshow())
            if fnlookup.lmexists(useName + "::" + token.tokText) {
                p.tokens[p.pos].subtype = subtypeUser
                return token.tokText, nil
            }

            // pf("  -- checking for c function name %s::%s in:\n%#v\n", useName, token.tokText, fnlookup.lmshow())
            //
            // Check C functions as fallback (highest overhead, lowest priority)
            // Use uc_match_c_func to respect use chain order instead of random map iteration
            namespaceName := uc_match_c_func(token.tokText)
            if namespaceName != "" {
                p.tokens[p.pos].subtype = subtypeUser
                return token.tokText, nil
            } else {
                // For namespaced calls, check if namespace matches a C library
                if p.prev.tokType == SYM_DoubleColon {
                    // fmt.Printf("[DEBUG] Namespaced call detected, namespace: '%s'\n", namespaceName)
                    p.tokens[p.pos].subtype = subtypeUser
                    return token.tokText, nil
                }
            }

        } else {
            // pf("(identifier) inside isFunc? check else clause. is a standard library function '%s'\n", token.tokText)
            p.tokens[p.pos].subtype = subtypeStandard
            // pf("(identifier) returning from identifier() at the isFunc? check else clause.\n")
            return token.tokText, nil
        }
    }
    // pf("(identifier) reached past func checks.\n")

    // Check for module constants FIRST (from AUTO clause)
    // This must happen before local/global lookup for qualified names (namespace::constant)
    if p.pos > 0 && p.prev.tokType == SYM_DoubleColon {
        // Qualified name: namespace provided explicitly
        // The namespace is in p.tokens[p.pos-2].tokText
        if p.pos >= 2 {
            namespaceName := p.tokens[p.pos-2].tokText
            moduleConstantsLock.RLock()
            if constMap, exists := moduleConstants[namespaceName]; exists {
                if val, found := constMap[token.tokText]; found {
                    moduleConstantsLock.RUnlock()
                    return val, nil
                }
            }
            moduleConstantsLock.RUnlock()

            // Also check C module idents if this is a C namespace
            // Find the function space for this module by checking cModuleAliasMap
            cModuleAliasMapLock.RLock()
            if fsid, exists := cModuleAliasMap[namespaceName]; exists {
                cModuleAliasMapLock.RUnlock()
                cModuleIdentsLock.RLock()
                if cIdent, cidExists := cModuleIdents[fsid]; cidExists {
                    // Linear search through the ident array for the constant
                    for i := range cIdent {
                        if cIdent[i].declared && cIdent[i].IName == token.tokText {
                            val := cIdent[i].IValue
                            cModuleIdentsLock.RUnlock()
                            return val, nil
                        }
                    }
                }
                cModuleIdentsLock.RUnlock()
            } else {
                cModuleAliasMapLock.RUnlock()
            }
        }
    } else {
        // Unqualified name: search USE chain
        // BUT: skip if next token is :: (this is a namespace part, not a constant lookup)
        if p.peek().tokType != SYM_DoubleColon {
            if _, val, found := uc_match_constant(token.tokText); found {
                return val, nil
            }
        }
    }

    // local variable lookup:
    bin := token.bindpos
    if bin >= uint64(len(*p.ident)) {
        newg := make([]Variable, bin+identGrowthSize)
        copy(newg, *p.ident)
        *p.ident = newg
    }

    // pf("(identifier) token binding position set to %d\n",bin)

    if (*p.ident)[bin].declared {
        // fmt.Printf("(il) fetched %s from local ident, bin %d :: %#v\n",token.tokText,bin,(*p.ident)[bin])
        return (*p.ident)[bin].IValue, nil
    }

    // pf("(identifier) token does not represent a variable, past .declared check\n")

    // global lookup:
    if val, there := vget(nil, p.mident, &mident, token.tokText); there {
        // fmt.Printf("(ig) fetched %s->%v from global ident\n",token.tokText,val)
        return val, nil
    }

    // permit module names
    if modlist[token.tokText] == true {
        // pf("(eval) permitting mod name %s\n",token.tokText)
        return nil, nil
    }

    // permit namespace:: names
    var ename string
    if p.prev.tokType != SYM_DoubleColon {
        if found := uc_match_enum(token.tokText); found != "" {
            ename = found + "::" + token.tokText
        } else {
            if len(p.namespace) > 0 {
                ename = p.namespace + "::" + token.tokText
            } else {
                ename = "main::" + token.tokText
            }
        }
    }
    // pf("ename is [%v]\n",ename)

    if enum[ename] != nil {
        // pf("(eval) permitting enum name %s\n",ename)
        return nil, nil
    }

    // Handle WITH ENUM context: resolve unqualified enum member names
    if p.inside_with_enum {
        fullEnumName := p.namespace + "::" + p.with_enum_name
        if enumDef, exists := enum[fullEnumName]; exists {
            if memberVal, memberExists := enumDef.members[token.tokText]; memberExists {
                return memberVal, nil
            }
        }
    }

    // permit references to uninitialised variables
    if permit_uninit {
        return nil, nil
    }

    // permit struct names
    sname := "anon"
    if token.tokText != "anon" {
        if p.prev.tokType != SYM_DoubleColon {
            if found := uc_match_struct(token.tokText); found != "" {
                sname = found + "::" + token.tokText
                // pf("sname->%s\n",sname)
            } else {
                if len(p.namespace) > 0 {
                    sname = p.namespace + "::" + token.tokText
                    // pf("sname->%s\n",sname)
                } else {
                    sname = "main::" + token.tokText
                }
            }
        }
    }
    // Resolve struct name through use_chain
    resolvedName := uc_match_struct(sname)
    lookupName := sname
    if resolvedName != "" {
        lookupName = resolvedName + "::" + sname
    }

    structmapslock.RLock()
    _, found := structmaps[lookupName]
    if !found {
        _, found = structmaps[sname]  // Fallback to exact lookup
    }
    structmapslock.RUnlock()

    if found || sname == "anon" {
        return sname, nil
    }

    panic(fmt.Errorf("'%s' is uninitialised.", token.tokText))

}

/*
 * Replacement variable handlers.
 */

// for locking vset/vcreate/vdelete during a variable write
var glock = &sync.RWMutex{}
var vlock = &sync.RWMutex{}

// inAutoProcessing indicates we're evaluating constants during AUTO clause processing
// When true, ev() won't call finish() on errors to avoid setting sig_int
var inAutoProcessing bool
var autoProcessingLock sync.RWMutex

func vunset(fs uint32, ident *[]Variable, name string) {
    bin := bind_int(fs, name)
    vlock.Lock()
    if (*ident)[bin].declared {
        (*ident)[bin] = Variable{declared: false}
    }
    vlock.Unlock()
}

func vdelete(fs uint32, ident *[]Variable, name string, ename string) {

    bin := bind_int(fs, name)
    vlock.RLock()
    decl := (*ident)[bin].declared
    vlock.RUnlock()
    if decl {
        m, _ := vget(nil, fs, ident, name)
        switch m := m.(type) {
        case map[string][]string:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string][]any:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]string:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]int:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]uint:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]float64:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]bool:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]*big.Int:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]*big.Float:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        case map[string]any:
            delete(m, ename)
            vset(nil, fs, ident, name, m)
        }
    }
}

func gvset(name string, value any) {
    glock.Lock()
    bin := bind_int(0, name)
    if bin >= uint64(len(gident)) {
        newg := make([]Variable, bin+identGrowthSize)
        copy(newg, gident)
        gident = newg
    }
    gident[bin].IName = name
    gident[bin].IValue = value
    gident[bin].declared = true
    glock.Unlock()
}

func vset(tok *Token, fs uint32, ident *[]Variable, name string, value any) {

    //    fmt.Printf("vset called for variable: %s with value: %#v (type: %T)\n", name, value, value)

    var bin uint64

    if fs < 3 && atomic.LoadInt32(&concurrent_funcs) > 0 {
        vlock.Lock()
        defer vlock.Unlock()
    }

    if tok == nil {
        bin = bind_int(fs, name)
        if bin >= uint64(len(*ident)) {
            newident := make([]Variable, bin+identGrowthSize)
            copy(newident, *ident)
            *ident = newident
        }
        (*ident)[bin] = Variable{IKind: 0, ITyped: false}
    } else {
        bin = tok.bindpos
    }

    if bin >= uint64(len(*ident)) {
        newident := make([]Variable, bin+identGrowthSize)
        copy(newident, *ident)
        *ident = newident
    }

    (*ident)[bin].IName = name
    (*ident)[bin].declared = true

    // struct type inference
    if value != nil {
        if reflect.TypeOf(value).Kind() == reflect.Struct {
            structName, count := struct_match(value)
            if count == 1 {
                (*ident)[bin].Kind_override = structName
            }
        }
    }

    if (*ident)[bin].ITyped {
        var ok bool
        switch (*ident)[bin].IKind {
        case kdynamic:
            // Dynamic multi-dimensional type - use reflection for type checking
            if (*ident)[bin].IValue != nil {
                targetType := reflect.TypeOf((*ident)[bin].IValue)
                valueType := reflect.TypeOf(value)
                if valueType != nil && valueType.AssignableTo(targetType) {
                    (*ident)[bin].IValue = value
                    ok = true
                }
            }
        case kbool:
            _, ok = value.(bool)
            if ok {
                (*ident)[bin].IValue = value
            }
        case kint:
            _, ok = value.(int)
            if ok {
                (*ident)[bin].IValue = value
            }
        case kuint:
            _, ok = value.(uint)
            if ok {
                (*ident)[bin].IValue = value
            }
        case kuint64:
            _, ok = value.(uint64)
            if ok {
                (*ident)[bin].IValue = value
            }
        case kfloat:
            _, ok = value.(float64)
            if ok {
                (*ident)[bin].IValue = value
            }

        case kbigi:
            switch value.(type) {
            case uint, uint32, int, int64, uint64, float64, *big.Int, *big.Float, string, uint8:
                (*ident)[bin].IValue.(*big.Int).Set(GetAsBigInt(value))
                ok = true
            }
        case kbigf:
            switch value.(type) {
            case uint, uint32, int, int64, uint64, float64, *big.Int, *big.Float, string, uint8:
                (*ident)[bin].IValue.(*big.Float).Set(GetAsBigFloat(value))
                ok = true
            }

        case kstring:
            _, ok = value.(string)
            if ok {
                (*ident)[bin].IValue = value
            }
        case kbyte:
            _, ok = value.(uint8)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksbool:
            _, ok = value.([]bool)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksint:
            _, ok = value.([]int)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksuint:
            _, ok = value.([]uint)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksfloat:
            _, ok = value.([]float64)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksstring:
            _, ok = value.([]string)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksbyte:
            _, ok = value.([]uint8)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksbigi:
            _, ok = value.([]*big.Int)
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksbigf:
            _, ok = value.([]*big.Float)
            if ok {
                (*ident)[bin].IValue = value
            }
        case kmap:
            _, ok = value.(map[string]any)
            if ok {
                (*ident)[bin].IValue = value
            }
        case kpointer:
            _, ok = value.(*CPointerValue)
            if !ok && value == nil {
                ok = true
            }
            if ok {
                (*ident)[bin].IValue = value
            }
        case ksany:
            _, ok = value.([]any)
            if ok {
                (*ident)[bin].IValue = value
            }
        }

        if !ok {
            panic(fmt.Errorf("invalid assignation : to type [%T] of [%T]", (*ident)[bin].IValue, value))
        }

    } else {
        // undeclared or untyped and needs replacing
        (*ident)[bin].IValue = value
    }

    return
}

func vgetElementi(fs uint32, ident *[]Variable, name string, el string) (any, bool) {
    var v any
    var ok bool
    v, ok = vget(nil, fs, ident, name)

    switch v := v.(type) {
    case map[string]int:
        return v[el], ok
    case map[string]float64:
        return v[el], ok
    case map[string][]string:
        return v[el], ok
    case map[string]string:
        return v[el], ok
    case map[string]bool:
        return v[el], ok
    case map[string][]any:
        return v[el], ok
    case map[string]any:
        return v[el], ok
    case map[string][]stackFrame:
        return v[el], ok
    case http.Header:
        return v[el], ok
    case []int:
        iel, _ := GetAsInt(el)
        return v[iel], ok
    case []bool:
        iel, _ := GetAsInt(el)
        return v[iel], ok
    case []float64:
        iel, _ := GetAsInt(el)
        return v[iel], ok
    case []string:
        iel, _ := GetAsInt(el)
        return v[iel], ok
    case string:
        iel, _ := GetAsInt(el)
        return string(v[iel]), ok
    case []stackFrame:
        iel, _ := GetAsInt(el)
        return v[iel], ok
    case []any:
        iel, _ := GetAsInt(el)
        return v[iel], ok
    default:
        // pf("Unknown type in %v[%v] (%T)\n",name,el,v)
        iel, _ := GetAsInt(el)
        for _, val := range reflect.ValueOf(v).Interface().([]any) {
            if iel == 0 {
                return val, true
            }
            iel -= 1
        }
    }
    return nil, false
}

func vsetElement(tok *Token, fs uint32, ident *[]Variable, name string, el any, value any) {

    var list any
    var ok bool

    if tok == nil {
        list, ok = vget(nil, fs, ident, name)
    } else {
        list, ok = vget(tok, fs, ident, name)
    }

    if !ok {
        list = make(map[string]any, LIST_SIZE_CAP)
        vset(nil, fs, ident, name, list)
    }

    bin := bind_int(fs, name)

    // pf("(vse) fs %v name %v bin %v listtype %T inbound value %+v\n",fs,name,bin,list,value)

    switch list.(type) {

    case map[string]string:
        var key string
        switch el.(type) {
        case int:
            key = intToString(el.(int))
        case float64:
            key = strconv.FormatFloat(el.(float64), 'f', -1, 64)
        case uint:
            key = strconv.FormatUint(uint64(el.(uint)), 10)
        case string:
            key = el.(string)
        }
        (*ident)[bin].IValue.(map[string]string)[key] = value.(string)
        // pf("(vse-s) set %v[%v] (key:%v) to %+v\n",name,el,key,(*ident)[bin].IValue.(map[string]string)[key])
        return

    case map[string][]any:
        var key string
        switch el.(type) {
        case int:
            key = intToString(el.(int))
        case float64:
            key = strconv.FormatFloat(el.(float64), 'f', -1, 64)
        case uint:
            key = strconv.FormatUint(uint64(el.(uint)), 10)
        case string:
            key = el.(string)
        }
        (*ident)[bin].IValue.(map[string][]any)[key] = value.([]any)
        // pf("(vse-aa) set %v[%v] (key:%v) to %+v\n",name,el,key,(*ident)[bin].IValue.(map[string][]any)[key])
        return

    case map[string]any:
        var key string
        switch el.(type) {
        case int:
            key = intToString(el.(int))
        case float64:
            key = strconv.FormatFloat(el.(float64), 'f', -1, 64)
        case uint:
            key = strconv.FormatUint(uint64(el.(uint)), 10)
        case string:
            key = el.(string)
        }
        (*ident)[bin].IValue.(map[string]any)[key] = value
        // pf("(vse-a) set %v[%v] (key:%v) to %+v\n",name,el,key,(*ident)[bin].IValue.(map[string]any)[key])
        return
    }

    // Handle kdynamic types using reflection (after map cases for performance)
    if (*ident)[bin].IKind == kdynamic {
        vt := reflect.TypeOf((*ident)[bin].IValue)
        val := reflect.ValueOf((*ident)[bin].IValue)

        // Ensure we have a slice or array
        if vt.Kind() != reflect.Slice && vt.Kind() != reflect.Array {
            panic(fmt.Errorf("Cannot index non-slice/array type %T", (*ident)[bin].IValue))
        }

        idx := el.(int)

        if idx < 0 || idx >= val.Len() {
            // For slices, extend if needed
            if vt.Kind() == reflect.Slice {
                newLen := idx + 1
                if newLen > val.Cap() {
                    newCap := val.Cap() * 2
                    if newCap == 0 {
                        newCap = 1
                    }
                    if newLen > newCap {
                        newCap = newLen
                    }
                    newSlice := reflect.MakeSlice(vt, newLen, newCap)
                    reflect.Copy(newSlice, val)
                    val = newSlice
                    (*ident)[bin].IValue = val.Interface()
                } else {
                    // pf("else clause:\nval:%#v\nlen:%v\ncap:%v\n",val,newLen,val.Cap())
                    // val.SetLen(newLen) // this won't work as not addressable
                    // so bodging it with a full copy for the moment:
                    newSlice := reflect.MakeSlice(vt, newLen, val.Cap())
                    reflect.Copy(newSlice, val)
                    val = newSlice
                    (*ident)[bin].IValue = val.Interface()
                    // ^^ this needs much improvement ^^
                }
            } else {
                panic(fmt.Errorf("Out of bounds access [element %d] on array of length %d", idx, val.Len()))
            }
        }

        // Set the value using reflection
        elemVal := reflect.ValueOf(value)
        targetElem := val.Index(idx)

        // Convert value to target type if needed
        if elemVal.Type() != targetElem.Type() {
            if elemVal.Type().ConvertibleTo(targetElem.Type()) {
                elemVal = elemVal.Convert(targetElem.Type())
            } else {
                panic(fmt.Errorf("Cannot convert value of type %T to element type %v", value, targetElem.Type()))
            }
        }

        targetElem.Set(elemVal)
        return
    }

    numel := el.(int)
    var fault bool

    switch (*ident)[bin].IValue.(type) {

    case string:
        if numel < 0 || numel >= len((*ident)[bin].IValue.(string)) {
            panic(fmt.Errorf("Out of bounds access [element %d] of %s", numel, name))
        }
        switch value.(type) {
        case string:
        default:
            panic(fmt.Errorf("Invalid type [%T] in string element access", value))
        }

        nv := (*ident)[bin].IValue.(string)
        switch len(nv) {
        case 1:
            (*ident)[bin].IValue = str.Join([]string{nv[:numel]}, string(value.(string)[0]))
        case 0:
            panic(fmt.Errorf("Assignee empty in element write"))
        default:
            (*ident)[bin].IValue = str.Join([]string{nv[:numel], nv[numel+1:]}, string(value.(string)[0]))
        }

    case []int:
        sz := cap((*ident)[bin].IValue.([]int))
        ll := len((*ident)[bin].IValue.([]int))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]int, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]int))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]int)[numel] = value.(int)

    case []uint8:
        sz := cap((*ident)[bin].IValue.([]uint8))
        ll := len((*ident)[bin].IValue.([]uint8))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]uint8, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]uint8))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]uint8)[numel] = value.(uint8)

    case []uint:
        sz := cap((*ident)[bin].IValue.([]uint))
        ll := len((*ident)[bin].IValue.([]uint))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]uint, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]uint))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]uint)[numel] = value.(uint)

    case []bool:
        sz := cap((*ident)[bin].IValue.([]bool))
        ll := len((*ident)[bin].IValue.([]bool))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]bool, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]bool))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]bool)[numel] = value.(bool)

    case []string:
        sz := cap((*ident)[bin].IValue.([]string))
        ll := len((*ident)[bin].IValue.([]string))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]string, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]string))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]string)[numel] = value.(string)

    case []float64:
        sz := cap((*ident)[bin].IValue.([]float64))
        ll := len((*ident)[bin].IValue.([]float64))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]float64, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]float64))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]float64)[numel], fault = GetAsFloat(value)
        if fault {
            panic(fmt.Errorf("Could not append to float array (ele:%v) a value '%+v' of type '%T'", numel, value, value))
        }

    case []*big.Int:
        sz := cap((*ident)[bin].IValue.([]*big.Int))
        ll := len((*ident)[bin].IValue.([]*big.Int))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]*big.Int, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]*big.Int))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]*big.Int)[numel] = GetAsBigInt(value)

    case []*big.Float:
        sz := cap((*ident)[bin].IValue.([]*big.Float))
        ll := len((*ident)[bin].IValue.([]*big.Float))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]*big.Float, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]*big.Float))
            (*ident)[bin].IValue = newar
        }
        (*ident)[bin].IValue.([]*big.Float)[numel] = GetAsBigFloat(value)

    case []any:
        sz := cap((*ident)[bin].IValue.([]any))
        ll := len((*ident)[bin].IValue.([]any))
        if numel >= sz || numel >= ll {
            newend := sz
            if numel >= sz {
                newend = sz * 2
            }
            if sz == 0 {
                newend = 1
            }
            if numel >= newend {
                newend = numel + 1
            }
            newar := make([]any, numel+1, newend)
            copy(newar, (*ident)[bin].IValue.([]any))
            (*ident)[bin].IValue = newar
        }
        if value == nil {
            (*ident)[bin].IValue.([]any)[numel] = nil
        } else {
            (*ident)[bin].IValue.([]any)[numel] = value.(any)
        }
    default:
        pf("DEFAULT: Unknown type %T for list %s\n", list, name)

    }

}

func gvget(name string) (any, bool) {
    bin := bind_int(0, name)
    if bin < uint64(len(gident)) && gident[bin].declared {
        glock.RLock()
        tv := gident[bin].IValue
        glock.RUnlock()
        return tv, true
    }
    return nil, false
}

func vget(token *Token, fs uint32, ident *[]Variable, name string) (any, bool) {
    var bin uint64
    if token == nil {
        bin = bind_int(fs, name)
    } else {
        bin = token.bindpos
    }

    if bin < uint64(len(*ident)) && (*ident)[bin].declared {
        return (*ident)[bin].IValue, true
    }
    return nil, false
}

func isBool(expr any) bool {
    switch reflect.TypeOf(expr).Kind() {
    case reflect.Bool:
        return true
    }
    return false
}

func isNumber(expr any) bool {
    switch reflect.TypeOf(expr).Kind() {
    case reflect.Float64, reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint8:
        return true
    }
    return false
}

/////////////////////////////////////////

func interpolate(ns string, fs uint32, ident *[]Variable, s string) string {
    if !interpolation || len(s) == 0 {
        return s
    }
    if str.IndexByte(s, '{') == -1 {
        return s
    }

    orig := s
    r := regexp.MustCompile(`{([^{}]*)}`)

    var interparse *leparser
    interparse = &leparser{}
    interparse.fs = fs
    interparse.ident = ident
    interparse.namespace = ns
    interparse.ctx = withProfilerContext(context.Background())
    interparse.interpolating = true
    if interactive {
        interparse.mident = 1
    } else {
        interparse.mident = 2
    }

    for {
        orig_s := s
        matches := r.FindAllStringSubmatch(s, -1)

        for _, v := range matches {
            kn := v[1]
            if kn[0] == '=' {
                continue
            }

            if kv, there := vget(nil, fs, ident, kn); there {
                switch kv.(type) {
                case int:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatInt(int64(kv.(int)), 10), -1)
                case int16:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatInt(int64(kv.(int16)), 10), -1)
                case float64:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatFloat(kv.(float64), 'g', -1, 64), -1)
                case bool:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatBool(kv.(bool)), -1)
                case string:
                    s = str.Replace(s, "{"+kn+"}", kv.(string), -1)
                case uint:
                    s = str.Replace(s, "{"+kn+"}", strconv.FormatUint(uint64(kv.(uint)), 10), -1)
                case []uint, []float64, []int, []bool, []any, []string:
                    s = str.Replace(s, "{"+kn+"}", sf("%v", kv), -1)
                case any:
                    s = str.Replace(s, "{"+kn+"}", sf("%v", kv), -1)
                default:
                    s = str.Replace(s, "{"+kn+"}", sf("!%T!%v", kv, kv), -1)
                }
            }
        }

        if orig_s == s {
            break
        }
    }

    var modified bool
    redo := true

    for redo {
        modified = false
        p := 0
        for ; p < len(s)-1; p += 1 {
            if s[p] == '{' && s[p+1] == '=' {
                nest := 0
                close_index := p
                for ; close_index < len(s); close_index += 1 {
                    if s[close_index] == '{' {
                        nest += 1
                    }
                    if s[close_index] == '}' {
                        nest -= 1
                    }
                    if s[close_index] == '}' && nest == 0 {
                        break
                    }
                }
                if nest > 0 {
                    break
                }

                if aval, err := ev(interparse, fs, s[p+2:close_index]); err == nil {
                    s = s[:p] + sf("%v", aval) + s[close_index+1:]
                    modified = true
                    break
                }
                p = close_index + 1
            }
        }
        if !modified {
            redo = false
        }
    }

    if s == "<nil>" {
        s = orig
    }

    return s
}

// evaluate an expression string
func ev(parser *leparser, fs uint32, ws string) (result any, err error) {

    // startTime:=time.Now()

    // build token list from string 'ws'
    toks := make([]Token, 0, len(ws)/3+1)
    var cl int16
    var p int
    var t *lcstruct
    for p = 0; p < len(ws); {
        t = nextToken(ws, fs, &cl, p)
        if t.carton.tokType == Identifier {
            t.carton.bindpos = bind_int(fs, t.carton.tokText)
            t.carton.bound = true
        }
        if t.tokPos != -1 {
            p = t.tokPos
        }
        toks = append(toks, t.carton)
        if t.eof {
            break
        }
    }

    // evaluate token list
    if len(toks) != 0 {
        result, err = parser.Eval(fs, toks)
    }

    if result == nil { // could not eval
        if err != nil {
            // During AUTO processing, don't call report() or finish()
            // Let the AUTO code handle error reporting
            autoProcessingLock.RLock()
            inAuto := inAutoProcessing
            autoProcessingLock.RUnlock()

            if !inAuto {
                parser.report(-1, sf("Error evaluating '%s'", ws))
                finish(false, ERR_EVAL)
            }
        }
    }

    if err != nil {
        if isNumber(ws) {
            var ierr bool
            result, ierr = GetAsInt(ws)
            if ierr {
                result, _ = GetAsFloat(ws)
            }
        }
    }

    return result, err

}

// / convert a token stream into a single expression struct
func crushEvalTokens(intoks []Token) ExpressionCarton {

    var crushedOpcodes str.Builder
    // crushedOpcodes.Grow(2)

    for t := range intoks {
        crushedOpcodes.WriteString(intoks[t].tokText)
    }

    return ExpressionCarton{text: crushedOpcodes.String(), assign: false, assignVar: ""}

}

/// the main call point for actor.go evaluation.
/// this function handles boxing the ev() call

func (p *leparser) wrappedEval(lfs uint32, lident *[]Variable, fs uint32, rident *[]Variable, tks []Token) (expr ExpressionCarton) {

    // search for any assignment operator +=,-=,*=,/=,%=
    // compound the terms beyond the assignment symbol and eval them.

    eqPos := -1
    hasComma := false
    var newEval []Token
    var err error

    if len(tks) == 2 {
        switch tks[1].tokType {
        case SYM_PP, SYM_MM:

            // override p.prev value as postIncDec uses it and we will be throwing
            //  away the p.* values shortly after this use.
            p.prev = tks[0]
            p.postIncDec(tks[1])
            expr.assign = true
            return expr
        }
    }

    standardAssign := true

floop1:
    for k, _ := range tks {
        if tks[k].tokType == O_Comma {
            hasComma = true
        }
        switch tks[k].tokType {
        // use whichever is encountered first
        case O_Assign:
            eqPos = k
            expr.result, err = p.Eval(fs, tks[k+1:])
            break floop1
        case SYM_PLE:
            expr.result, err = p.Eval(fs, tks[k+1:])
            if err == nil {
                eqPos = k
                newEval = make([]Token, len(tks[:k])+2)
                copy(newEval, tks[:k])
                newEval[k] = Token{tokType: O_Plus}
            }
            standardAssign = false
            break floop1
        case SYM_MIE:
            expr.result, err = p.Eval(fs, tks[k+1:])
            if err == nil {
                eqPos = k
                newEval = make([]Token, len(tks[:k])+2)
                copy(newEval, tks[:k])
                newEval[k] = Token{tokType: O_Minus}
            }
            standardAssign = false
            break floop1
        case SYM_MUE:
            expr.result, err = p.Eval(fs, tks[k+1:])
            if err == nil {
                eqPos = k
                newEval = make([]Token, len(tks[:k])+2)
                copy(newEval, tks[:k])
                newEval[k] = Token{tokType: O_Multiply}
            }
            standardAssign = false
            break floop1
        case SYM_DIE:
            expr.result, err = p.Eval(fs, tks[k+1:])
            if err == nil {
                eqPos = k
                newEval = make([]Token, len(tks[:k])+2)
                copy(newEval, tks[:k])
                newEval[k] = Token{tokType: O_Divide}
            }
            standardAssign = false
            break floop1
        case SYM_MOE:
            expr.result, err = p.Eval(fs, tks[k+1:])
            if err == nil {
                eqPos = k
                newEval = make([]Token, len(tks[:k])+2)
                copy(newEval, tks[:k])
                newEval[k] = Token{tokType: O_Percent}
            }
            standardAssign = false
            break floop1
        }
    }

    if eqPos == -1 {
        expr.result, err = p.Eval(fs, tks)
        expr.assignPos = -1
    } else {
        expr.assign = true
        expr.assignPos = eqPos

        // before eval, rewrite lhs token bindings to their lhs equivalent
        if !standardAssign {
            if lfs != fs {
                if newEval[0].tokType == Identifier {
                    if !(*lident)[newEval[0].bindpos].declared {
                        p.report(-1, "you may only amend existing variables outside of local scope")
                        expr.evalError = true
                        finish(false, ERR_SYNTAX)
                        return expr
                    }
                }
            }
            switch expr.result.(type) {
            case string:
                newEval[eqPos+1] = Token{tokType: StringLiteral, tokText: expr.result.(string), tokVal: expr.result}
            default:
                newEval[eqPos+1] = Token{tokType: NumericLiteral, tokText: "", tokVal: expr.result}
            }

            expr.result, err = p.Eval(lfs, newEval)

        }
    }

    if err != nil {
        expr.evalError = true
        expr.errVal = err
        return expr
    }

    if expr.assign {
        // pf("[#4]Assigning : lfs %d rfs %d toks->%+v[#-]\n",lfs,fs,tks)
        // pf("[#5]This expression box result address -> %v\n",&expr.result)
        p.doAssign(lfs, lident, fs, rident, tks, &expr, eqPos, hasComma)
    }

    return expr

}

func (p *leparser) tryOperator(left any, right any) any {
    // The ?? operator converts various failure conditions into exceptions
    // If left is successful, return it; otherwise throw an exception with the right category

    var shouldThrow bool

    // Check if left indicates a failure condition

    switch v := left.(type) {
    case nil:
        // nil result - convert to exception
        shouldThrow = true

    case struct {
        Out  string
        Err  string
        Code int
        Okay bool
    }:
        // Shell command result from {...}
        if !v.Okay {
            shouldThrow = true
        }

    case bool:
        // Boolean result - false is considered failure
        if !v {
            shouldThrow = true
        }

    case error:
        // Already an error - convert to exception
        shouldThrow = true

    case string:
        // Empty string is considered failure
        if v == "" {
            shouldThrow = true
        }

    case int:
        // Zero is considered failure
        if v == 0 {
            shouldThrow = true
        }

    case float64:
        // Zero is considered failure
        if v == 0.0 {
            shouldThrow = true
        }
    }

    if shouldThrow {
        // Convert right to string for exception category
        category := ""
        switch r := right.(type) {
        case string:
            category = r
        case nil:
            category = "error"
        default:
            category = sf("%v", r)
        }

        // Check error style setting to determine how to handle the failure
        errorStyleLock.RLock()
        currentErrorStyle := errorStyleMode
        errorStyleLock.RUnlock()

        if currentErrorStyle == ERROR_STYLE_EXCEPTION || currentErrorStyle == ERROR_STYLE_MIXED {
            // Throw an ExceptionThrow for exception handling
            panic(ExceptionThrow{
                Category: category,
                Message:  sf("?? operator failure: %v -> %s", left, category),
            })
        } else {
            // Panic with regular error for panic mode
            panic(fmt.Errorf("?? operator failure: %v -> %s", left, category))
        }
    }

    // No failure, return the original value
    return left
}
