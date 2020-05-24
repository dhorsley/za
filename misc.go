package main

import (
	"log"
	"os"
	"regexp"
	str "strings"
)

// does file exist?
func fexists(fp string) bool {
    f,err:=os.Stat(fp)
    if err==nil {
        return f.Mode().IsRegular()
    }
    return false
}

func getReportFunctionName(ifs uint64) string {
	nl,_ := numlookup.lmget(ifs)
	/*
	add := ""
    if str.IndexByte(nl, '@') > -1 {
		add = nl[:str.IndexByte(nl, '@')]
	} else {
		add = nl
	}
	add = add + " @ "
	return add
    */
    return nl
}

func report(ifs uint64, pos int, s string) {
    add := getReportFunctionName(ifs)
    // err_stream:=os.Stderr
    // if interactive { err_stream=os.Stdout }
	if pos > 0 {
		// fpf(err_stream, sparkle(sf("\n[#bred][#7]Error in %s,line %d[##][#-]\n%s\n", add, pos, s)))
		pf(sparkle(sf("\n[#bred][#7]Error in %s,line %d[##][#-]\n%s\n", add, pos, s)))
	} else {
	    nl,_ := numlookup.lmget(ifs)
		// fpf(err_stream, sparkle(sf("\n[#bred][#7]Error in %s[##][#-]\n%s\n", nl, s)))
		pf(sparkle(sf("\n[#bred][#7]Error in %s[##][#-]\n%s\n", nl, s)))
	}
}

func appendToTestReport(test_output_file string, ifs uint64, pos int, s string) {

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
	pf(spf(globalspace, "{@language} version {@version} - built for {@ct_info}\n"))
    pf(spf(globalspace, "[#1]Last build: {@creation_date}[#-]\n"))

}

func help(hargs string) {

	switch len(hargs) {
	case 0:
		helppage := `
[#1]{lower("{@language}")} [-v] [-h] [-i] [-m] [-c] [-C] [-l] [-S]      \
    [-s [#i1]path[#i0]] [-t] [-O [#i1]tval[#i0]]                    \
    [-G [#i1]group_filter[#i0]]  [-o [#i1]output_file[#i0]]         \
    [-r] [-F "[#i1]sep[#i0]"] [-e [#i1]program_string[#i0]]         \
    [-T [#i1]time-out[#i0]] [[-f] [#i1]input_file[#i0]][#-]

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
    [#4]-c[#-] : Ignore colour code macros at startup
    [#4]-C[#-] : Enable colour code macros at startup
    [#4]-l[#-] : Enable mutex locking for multi-threaded use
    [#4]-s[#-] : Provide an alternative path for the co-process shell
    [#4]-S[#-] : Disable the co-process shell
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
[#5]ENDDEF[#-]                                          - end a function definition.
[#5]SHOWDEF [ [#i1]name[#i0] ][#-]                                - display a single function definition, or all functions.
[#5]RETURN [#i1]retval[#i0][#-]                                   - return from function, with value.
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
[#4]CONTINUE[#-]                                        - proceed to next loop iteration immediately.
[#4]EXIT [#i1]code[#i0][#-]                                       - exit script with status code.
[#2]PRINT[LN] [#i1]expression [ , expression ][#i0][#-]           - local echo. (PRINTLN adds a trailing newline character.)
[#2]LOG [#i1]expression[#i0][#-]                                  - local echo plus pre-named destination log file.
[#2]LOGGING OFF | ON [#i1]name[#i0][#-]                           - disable or enable logging and specify the log file name.
[#2]LOGGING QUIET | LOUD[#-]                            - option to squash console echo of LOG messages.
[#2]CLS [ [#i1]pane_id[#i0] ][#-]                                 - clear console screen/pane.
[#2]AT [#i1]row,column[#i0][#-]                                   - move cursor to [#i1]row,column[#i0].
[#2]PANE DEFINE [#i1]name,row,col,w,h[,title[,border]][#i0][#-]   - Define a new coordinate pane.
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
[#7]UNSET[#-] [#i1]var[#i0]                                       - destroy variable allocation for [#i1]var[#i0].
[#7]INIT[#-] [#i1]var[#i0] [#i1]type[#i0] [[#i1]size[#i0]]                            - Dimension an array. Type can be bool, int, float, mixed or assoc.
[#7]ZERO[#-] [#i1]var[#i0]                                        - set [#i1]var[#i0] to zero
[#7]INC[#-] [#i1]var[#i0] [ [#i1]step[#i0] ]                                - increment [#i1]var[#i0] by 1 (or [#i1]step[#i0])
[#7]DEC[#-] [#i1]var[#i0] [ [#i1]step[#i0] ]                                - decrement [#i1]var[#i0] by 1 (or [#i1]step[#i0])
[#7]PAUSE[#-] [#i1]timer_ms[#i0]                                  - delay [#i1]timer_ms[#i0] milliseconds.
[#7]NOP[#-]                                             - dummy 100 millisecond command.
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
