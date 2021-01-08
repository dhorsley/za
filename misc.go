package main

import (
	"log"
    "fmt"
	"os"
	"regexp"
	str "strings"
)


var openLocks int

// deprecated: just locking everything now.

// enable or disable global locking
func locks(b bool) {

    globlock.Lock()

    if b {
        openLocks++
        lockSafety=true
        // pf("[ol] increased to %d\n",openLocks)
    }
    if !b && openLocks>0 {
        openLocks--
        // pf("[ol] reduced to %d\n",openLocks)
    }

    if openLocks==0 && !b {
        lockSafety=false
        // pf("[ol] locks off\n")
    }

    globlock.Unlock()

}


// does file exist?
func fexists(fp string) bool {
    f,err:=os.Stat(fp)
    if err==nil {
        return f.Mode().IsRegular()
    }
    return false
}

func getReportFunctionName(ifs uint32, full bool) string {
	nl,_ := numlookup.lmget(ifs)
    if !full && str.IndexByte(nl, '@') > -1 {
		nl=nl[:str.IndexByte(nl, '@')]
    }
    return nl
}

/*
func ShowSource(funcInstance string,start int,end int) bool {

    var funcInstanceId uint32
    var present bool

    // return if func instance doesn't exist
    if funcInstanceId, present = fnlookup.lmget(funcInstance); !present {
        return false
    }

    if funcInstanceId < uint32(len(functionspaces)) {

        var baseId uint32

        // convert instance to source function
        funcName := getReportFunctionName(funcInstanceId,false)    //      -> name of failing func converted to base name
        if funcInstanceId==2 {
            baseId=1
        } else {
            baseId,_    = fnlookup.lmget(funcName)                 //      -> id of base funcs source space 
        }

        if start<0 { start=0 }
        max:=len(functionspaces[baseId])-1
        if end+1>max { end=max-1 }

        strOut:="[#CTE]"

        for q := range functionspaces[baseId][start:end+1] {
            strOut = strOut + sf("%5d : %s\n[#CTE]", start+q, sourceStore[baseId][start+q])
        }

        strOut = sf("\n[#CTE]%s(%v)\n[#CTE]", funcName, str.Join(functionArgs[baseId].args, ",")) + strOut
        pf(strOut)

    }
    return true
}
*/

func showCallChain(base string) {

    // show chain
    pf("[#CTE][#5]")
    for k,v:=range callChain {
        if k==0 { continue }
        v.name=getReportFunctionName(v.loc,false)
        pf("-> %s (%d) (%s) ",v.name,v.line,lookupChainName(v.registrant))
    }
    pf("-> [#6]"+base+"[#-]\n[#CTE]")

    // show source lines - defunct
    /*
    for k,v:=range callChain {
        if k==0 { continue }
        v.name=getReportFunctionName(v.loc,false)
        ShowSource(v.name,v.line,v.line)
    }
    */
}

func lookupChainName(n uint8) string {
    //  ciTrap ciCall ciEval ciMod ciAsyn ciRepl ciRhsb ciLnet ciMain ciErr
    return [10]string{"0-Trap Handler","1-Call","2-Evaluator",
                            "3-Module Definition","4-Async Handler","5-Interactive Mode",
                            "6-UDF Builder","7-Net Library","8-Main Function","9-Error Handling"}[n]
}

func (parser *leparser) report(s string) {

    var baseId uint32

    line:=parser.line
    ifs:=parser.fs
                                                    // ifs  -> id of failing func
    funcName    := getReportFunctionName(ifs,false) //      -> name of failing func
    if ifs==2 {
        baseId=1
    } else {
        baseId,_ = fnlookup.lmget(funcName)         //      -> id of base func  
    }
    baseName,_  := numlookup.lmget(baseId)          //      -> name of base func

    // si:=1 // default source index
    //if sm,ok:=sourceMap[baseId]; ok {
        // found a mapping for parent that created a func, and thus holds source
    //    si=int(sm)
    //}

    var line_content string
    if len(functionspaces[baseId])>0 {
        if baseId!=0 {
            // line_content=sourceStore[si][line]
            line_content=functionspaces[baseId][parser.stmtline].Original
        } else {
            line_content="Interactive Mode"
        }
    }

    fmt.Print( sparkle( sf("[#CTE]\n[#bred]\n[#CTE]"+
        "[#7]Error in %+v/%s (line #%d) : ", fileMap[sourceMap[baseId]],baseName,line+1))+
        line_content+"\n"+
        sparkle("[##][#-][#CTE]")+
        sf("%s\n", s)+sparkle("[#CTE]"))

    if !interactive {
        showCallChain(baseName)
        pf("\n[#CTE]")
    }

}


func appendToTestReport(test_output_file string, ifs uint32, pos int, s string) {

	s = sparkle(s) + "\n"

	f, err := os.OpenFile(test_output_file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := f.Write([]byte(s)); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

}

func version() {
	pf(spf(0, "{@language} version {@version} - built for {@ct_info}\n"))
    pf(spf(0, "[#1]Last build: {@creation_date}[#-]\n"))

}

func help(hargs string) {

	switch len(hargs) {
	case 0:
		helppage := `
[#1]za [-v] [-h] [-i] [-m] [-c] [-C] [-Q] [-S]      \
    [-s [#i1]path[#i0]] [-t] [-O [#i1]tval[#i0]]                    \
    [-G [#i1]group_filter[#i0]]  [-o [#i1]output_file[#i0]]         \
    [-r] [-F "[#i1]sep[#i0]"] [-e [#i1]program_string[#i0]]         \
    [-T [#i1]time-out[#i0]] [-U [#i1]sep[#i0]] [[-f] [#i1]input_file[#i0]][#-]

    [#4]-v[#-] : Version
    [#4]-h[#-] : Help
    [#4]-f[#-] : Process script [#i1]input_file[#i0]
    [#4]-i[#-] : Interactive mode
    [#4]-t[#-] : Test mode
    [#4]-O[#-] : Test override value [#i1]tval[#i0]
    [#4]-o[#-] : Name the test file [#i1]output_file[#i0]
    [#4]-G[#-] : Test group filter [#i1]group_filter[#i0]
    [#4]-T[#-] : Sets the [#i1]time-out[#i0] duration, in milliseconds, for calls to the co-process shell
    [#4]-m[#-] : Mark co-process command progress
    [#4]-U[#-] : Specify system command separator byte
    [#4]-c[#-] : Ignore colour code macros at startup
    [#4]-C[#-] : Enable colour code macros at startup
    [#4]-s[#-] : Provide an alternative path for the co-process shell
    [#4]-S[#-] : Disable the co-process shell
    [#4]-Q[#-] : Show shell command options
    [#4]-e[#-] : Provide source code in a string for interpretation. Stdin becomes available for data input
    [#4]-r[#-] : Wraps a -e argument in a loop iterating standard input. Each line is automatically split into fields
    [#4]-F[#-] : Provides a field separator character for -r

    Please consult the za-reference document or execute commands() for a command list.
    A list of library functions is available with the funcs(filter_string) call.

`

		gpf(helppage)

	default:

		foundCommand := false
		foundFunction := false

		cmd := str.ToLower(hargs)
		cmdMatchList := ""
		funcMatchList := ""

		// check for keyword first:
		re, err := regexp.Compile(`(^|\n){1}\[#[0-9]\]` + cmd + `.*?\n`)

		if err == nil {
			cmdMatchList = sparkle(str.TrimSpace(re.FindString(str.ToLower(cmdpage))))
			remspace, _ := regexp.Compile(`[ ]+`)
			cmdMatchList = remspace.ReplaceAllString(cmdMatchList, " ")
			if cmdMatchList != "" {
				foundCommand = true
			}
		}

		// check for stdlib if not a keyword.
		if _, exists := slhelp[cmd]; exists {
			lhs := slhelp[cmd].out
			colour := "2"
			if slhelp[cmd].out != "" {
				lhs += " = "
				colour = "3"
			}
			params := slhelp[cmd].in
			funcMatchList += sf(sparkle("[#"+colour+"]%s%s(%s)[#-]\n"), lhs, cmd, params)
			funcMatchList += sparkle(sf("[#4]%s[#-]", slhelp[cmd].action))
			foundFunction = true
		}

		if foundFunction || foundCommand {
			if foundCommand {
				pf("keyword  : %v\n", cmdMatchList)
			}
			if foundFunction {
				pf("function : %v\n", funcMatchList)
			}
		}

	}
}

var cmdpage string = `
Available commands:
[#5]DEFINE [#i1]name[#i0] ([#i1]arg1,...,argN[#i0])[#-]                     - create a function.
[#5]END[#-]                                             - end a function definition.
[#5]RETURN [#i1]retval[#i0][#-]                                   - return from function, with value.
[#5]ASYNC [#i1]handle_map f(...)[#i0] [[#i1]handle_id[#i0]][#-]             - run a function asynchronously.
[#4]ON [#i1]condition[#i0] DO [#i1]command[#i0][#-]                         - perform a single command if condition evaluates to true.
[#4]IF [#i1]condition[#i0][#-]                                    - test condition and start execution block if true.
[#4]ELSE[#-]                                            - start execution block for false state.
[#4]ENDIF[#-]                                           - terminate IF execution block.
[#4]WHILE [#i1]condition[#i0][#-]                                 - start while...end loop block.
[#4]ENDWHILE[#-]                                        - end of while...end loop block.
[#4]FOR [#i1]var[#i0] = [#i1]start[#i0] TO [#i1]end[#i0] [ STEP [#i1]step[#i0] ][#-]            - start FOR loop block. (integer iteration only)
[#4]FOREACH [#i1]var[#i0] IN [#i1]var[#i0] | [#i1]fn(expr)[#i0] | [#i1]"literal"[#i0][#-]       - iterate over variable content lines.
[#4]ENDFOR[#-]                                          - terminate FOR execution block.
[#4]WHEN [#i1]expr[#i0][#-]                                       - switch-like structure.
[#4]IS | CONTAINS [#i1]expr[#-][#i0]                              - when [#i1]expr[#i0] matches value or regex.
[#4]OR[#-]                                              - default WHEN case.
[#4]ENDWHEN[#-]                                         - terminates the WHEN block.
[#4]BREAK[#-]                                           - exit a loop or WHEN clause immediately.
[#4]WITH [#i1]var[#i0] AS [#i1]name[#i0][#-]                                - starts a WITH construct.
[#4]ENDWITH[#-]                                         - ends a WITH construct.
[#4]CONTINUE[#-]                                        - proceed to next loop iteration immediately.
[#4]EXIT [#i1]code[#i0][#-]                                       - exit script with status code.
[#2]PRINT[LN] [#i1]expression [ , expression ][#i0][#-]           - local echo. (PRINTLN adds a trailing newline character.)
[#2]LOG [#i1]expression[#i0][#-]                                  - local echo plus pre-named destination log file.
[#2]LOGGING OFF | ON [#i1]name[#i0][#-]                           - disable or enable logging and specify the log file name.
[#2]LOGGING QUIET | LOUD[#-]                            - option to squash console echo of LOG messages.
[#2]CLS [ [#i1]pane_id[#i0] ][#-]                                 - clear console screen/pane.
[#2]AT [#i1]row,column[#i0][#-]                                   - move cursor to [#i1]row,column[#i0].
[#2]PANE DEFINE [#i1]name,row,col,h,w[,title[,border]][#i0][#-]   - Define a new coordinate pane.
[#2]PANE SELECT [#i1]name[#i0][#-]                                - Select a defined pane as active.
[#2]PANE OFF[#-]                                        - Disable panes.
[#6]REQUIRE [#i1]feature[#i0] [ [#i1]num[#i0] ][#-]                         - assert feature availability and optional version level, or exit.
[#6]INPUT [#i1]id[#i0] [#i1]type[#i0] [#i1]position[#i0][#-]                          - set variable [#i1]id[#i0] from external value or exits.
[#6]PROMPT [#i1]var prompt[#i0] [ [#i1]validator[#i0] ][#-]                 - set [#i1]var[#i0] from stdin. loops until [#i1]validator[#i0] satisfied.
[#3]MODULE [#i1]modname[#i0][#-]                                  - reads in state from a module file.
[#3]TEST [#i1]name[#i0] GROUP [#i1]gname[#i0] [ ASSERT FAIL|CONTINUE ][#-]  - Define a test
[#3]ENDTEST[#-]                                         - End a test definition
[#3]ASSERT [#i1]condition[#i0][#-]                                - Confirm condition is true, or exit. In test mode, asserts should instead be collected.
[#3]DOC [ [#i1]function_name[#i0] ] [#i1]comment[#i0][#-]                   - Create an exportable comment, for the documentation generator.
[#7]INIT[#-] [#i1]var[#i0] [#i1]type[#i0] [[#i1]size[#i0]]                            - Dimension an array. Type can be bool, int, float, mixed or assoc.
[#7]VAR[#-] [#i1]var type[#i0]                                    - declare an optional type.
[#7]PAUSE[#-] [#i1]timer_ms[#i0]                                  - delay [#i1]timer_ms[#i0] milliseconds.
[#7]NOP[#-]                                             - dummy 100 millisecond command.
[#7]STRUCT[#-] [#i1]name[#i0]                                     - begin structure definition.
[#7]ENDSTRUCT[#-]                                       - end structure definition.
[#7]SHOWSTRUCT[#-]                                      - display structure definitions.
[#7]|[#-] [#i1]command[#i0]                                       - execute shell command.
[#7]VERSION[#-]                                         - show Za version.
[#7]HELP[#-]                                            - this page.

[#i1]name[#i0][#i1](params)[#i0]                                    - call a function, with parameters <params>
[#i1]var[#i0] = [#i1]value[#i0]                                     - assign to variable.
[#i1]var[#i0] =| [#i1]expression[#i0]                               - store result of a local shell command to variable.

# comment                                       - comment to end of line.

`

func commands() {
	gpf(cmdpage)
}
