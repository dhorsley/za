//go:build !test

package main

import (
	"errors"
	"fmt"
	"os"
)

// Global state for error handling is now in globalErrorContext

func buildErrorLib() {

	// error handling

	features["error"] = Feature{version: 1, category: "error"}
	categories["error"] = []string{"error_message", "error_source_location", "error_source_context",
		"error_call_chain", "error_call_stack", "error_local_variables", "error_global_variables",
		"error_default_handler", "error_extend", "error_emergency_exit", "error_filename"}

	slhelp["error_message"] = LibHelp{in: "", out: "string", action: "Returns the original error message text."}
	stdlib["error_message"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return "", errors.New("error_message() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_message", args, 1, "0"); !ok {
			return nil, err
		}

		return globalErrorContext.Message, nil
	}

	slhelp["error_source_location"] = LibHelp{in: "", out: "map", action: "Returns error location as {file, line, function, module}."}
	stdlib["error_source_location"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return map[string]any{}, errors.New("error_source_location() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_source_location", args, 1, "0"); !ok {
			return nil, err
		}

		// Get the actual filename from fileMap
		filename := globalErrorContext.ModuleName
		if fileMapValue, exists := fileMap.Load(evalfs); exists {
			filename = fileMapValue.(string)
		}

		return map[string]any{
			"file":     filename,
			"line":     int(globalErrorContext.SourceLine),
			"function": globalErrorContext.FunctionName,
			"module":   globalErrorContext.ModuleName,
		}, nil
	}

	slhelp["error_source_context"] = LibHelp{in: "int,int", out: "[]string", action: "Returns source lines (before, after) around error."}
	stdlib["error_source_context"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return []string{}, errors.New("error_source_context() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_source_context", args, 1, "2", "int", "int"); !ok {
			return nil, err
		}

		before := args[0].(int)
		after := args[1].(int)

		// Return subset of source lines based on before/after parameters
		lines := globalErrorContext.SourceLines
		if len(lines) == 0 {
			return []string{}, nil
		}

		// Find current line in context (usually middle of the array)
		currentIndex := len(lines) / 2
		startIndex := max(0, currentIndex-before)
		endIndex := min(len(lines)-1, currentIndex+after)

		result := make([]string, 0)
		for i := startIndex; i <= endIndex; i++ {
			result = append(result, lines[i])
		}

		return result, nil
	}

	slhelp["error_source_line_numbers"] = LibHelp{in: "int,int", out: "[]int", action: "Returns source line numbers (before, after) around error."}
	stdlib["error_source_line_numbers"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return []int{}, errors.New("error_source_line_numbers() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_source_line_numbers", args, 1, "2", "int", "int"); !ok {
			return nil, err
		}

		before := args[0].(int)
		after := args[1].(int)

		// Use the same logic as error_source_context to ensure matching arrays
		lines := globalErrorContext.SourceLines
		if len(lines) == 0 {
			return []int{}, nil
		}

		// Find current line in context (usually middle of the array)
		currentIndex := len(lines) / 2
		startIndex := max(0, currentIndex-before)
		endIndex := min(len(lines)-1, currentIndex+after)

		// Calculate the corresponding line numbers
		// The source lines were collected in a ±2 range around the error line
		errorLine := int(globalErrorContext.SourceLine)

		// The middle of the source lines array corresponds to the error line
		result := make([]int, 0)
		for i := startIndex; i <= endIndex; i++ {
			// Calculate line number: error line + (index - middle index)
			lineNumber := errorLine + (i - currentIndex)
			result = append(result, lineNumber)
		}

		return result, nil
	}

	slhelp["error_call_chain"] = LibHelp{in: "", out: "[]map", action: "Returns full call chain with function names and arguments."}
	stdlib["error_call_chain"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return []map[string]any{}, errors.New("error_call_chain() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_call_chain", args, 1, "0"); !ok {
			return nil, err
		}

		return globalErrorContext.CallChain, nil
	}

	slhelp["error_call_stack"] = LibHelp{in: "", out: "[]string", action: "Returns function names in call chain."}
	stdlib["error_call_stack"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return []string{}, errors.New("error_call_stack() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_call_stack", args, 1, "0"); !ok {
			return nil, err
		}

		return globalErrorContext.CallStack, nil
	}

	slhelp["error_local_variables"] = LibHelp{in: "", out: "map", action: "Returns variables in the error frame."}
	stdlib["error_local_variables"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return map[string]any{}, errors.New("error_local_variables() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_local_variables", args, 1, "0"); !ok {
			return nil, err
		}

		return globalErrorContext.LocalVars, nil
	}

	slhelp["error_global_variables"] = LibHelp{in: "", out: "map", action: "Returns user-defined global variables (same as mdump)."}
	stdlib["error_global_variables"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return map[string]any{}, errors.New("error_global_variables() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_global_variables", args, 1, "0"); !ok {
			return nil, err
		}

		return globalErrorContext.GlobalVars, nil
	}

	slhelp["error_default_handler"] = LibHelp{in: "", out: "", action: "Calls the default Za error handler."}
	stdlib["error_default_handler"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return nil, errors.New("error_default_handler() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_default_handler", args, 1, "0"); !ok {
			return nil, err
		}

		// Call default error handler - just print the message and exit
		pf("Error: %s\n", globalErrorContext.Message)
		finish(false, ERR_EVAL)
		return nil, nil
	}

	slhelp["error_extend"] = LibHelp{in: "bool", out: "", action: "Enable/disable enhanced error context display."}
	stdlib["error_extend"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("error_extend", args, 1, "1", "bool"); !ok {
			return nil, err
		}

		enhancedErrorsEnabled = args[0].(bool)
		return nil, nil
	}

	slhelp["error_emergency_exit"] = LibHelp{in: "int", out: "", action: "Emergency exit with specified code."}
	stdlib["error_emergency_exit"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return nil, errors.New("error_emergency_exit() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_emergency_exit", args, 1, "1", "int"); !ok {
			return nil, err
		}

		exitCode := args[0].(int)
		os.Exit(exitCode)
		return nil, nil
	}

	slhelp["error_filename"] = LibHelp{in: "", out: "string", action: "Returns the filename where the error occurred."}
	stdlib["error_filename"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if !globalErrorContext.InErrorHandler {
			return "", errors.New("error_filename() can only be called from within an error handler")
		}
		if ok, err := expect_args("error_filename", args, 1, "0"); !ok {
			return nil, err
		}

		// Get the actual filename from fileMap
		filename := globalErrorContext.ModuleName
		if fileMapValue, exists := fileMap.Load(evalfs); exists {
			filename = fileMapValue.(string)
		}

		return filename, nil
	}

	slhelp["error_style"] = LibHelp{in: "mode_string", out: "string", action: "Set error handling style mode and return previous mode.\n[#SOL]panic: standard Go panic/recover (default)\n[#SOL]exception: convert panics to exceptions\n[#SOL]mixed: both panic and exception handling"}
	stdlib["error_style"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("error_style", args, 2, "1", "string", "0"); !ok {
			return nil, err
		}

		errorStyleLock.Lock()
		defer errorStyleLock.Unlock()

		// Return current mode as string
		var currentMode string
		switch errorStyleMode {
		case ERROR_STYLE_PANIC:
			currentMode = "panic"
		case ERROR_STYLE_EXCEPTION:
			currentMode = "exception"
		case ERROR_STYLE_MIXED:
			currentMode = "mixed"
		default:
			currentMode = "panic"
		}

		// If no arguments, just return current mode
		if len(args) == 0 {
			return currentMode, nil
		}

		// Set new mode
		newMode := args[0].(string)
		switch newMode {
		case "panic":
			errorStyleMode = ERROR_STYLE_PANIC
		case "exception":
			errorStyleMode = ERROR_STYLE_EXCEPTION
		case "mixed":
			errorStyleMode = ERROR_STYLE_MIXED
		default:
			return currentMode, fmt.Errorf("Invalid error style mode: %s (use: panic, exception, mixed)", newMode)
		}

		return currentMode, nil
	}

	// Exception logging functions
	slhelp["log_exception"] = LibHelp{in: "any,string", out: "", action: "Log an exception to the current logging target."}
	stdlib["log_exception"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("log_exception", args, 1, "2", "any", "string"); !ok {
			return nil, err
		}

		category := args[0]
		message := args[1].(string)

		// Get current function and line info
		functionName := "unknown"
		lineNumber := int16(0)

		// Try to get function name from current context
		if evalfs > 0 && evalfs < uint32(len(calltable)) {
			if name, exists := numlookup.lmget(calltable[evalfs].caller); exists {
				functionName = name
			}
		}

		// Log the exception
		logException(category, message, int(lineNumber), functionName, nil)
		return nil, nil
	}

	slhelp["log_exception_with_stack"] = LibHelp{in: "any,string,[]any", out: "", action: "Log an exception with custom stack trace data."}
	stdlib["log_exception_with_stack"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("log_exception_with_stack", args, 1, "3", "any", "string", "[]any"); !ok {
			return nil, err
		}

		category := args[0]
		message := args[1].(string)
		stackData := args[2].([]any)

		// Convert stack data to stackFrame slice
		var stackTrace []stackFrame
		for _, frameAny := range stackData {
			frame, ok := frameAny.(map[string]any)
			if !ok {
				continue // Skip invalid frame data
			}

			function := "unknown"
			if f, ok := frame["function"].(string); ok {
				function = f
			}

			line := int16(0)
			if l, ok := frame["line"].(int); ok {
				line = int16(l)
			}

			caller := ""
			if c, ok := frame["caller"].(string); ok {
				caller = c
			}

			stackTrace = append(stackTrace, stackFrame{
				function: function,
				line:     line,
				caller:   caller,
			})
		}

		// Get current function name
		functionName := "unknown"
		if evalfs > 0 && evalfs < uint32(len(calltable)) {
			if name, exists := numlookup.lmget(calltable[evalfs].caller); exists {
				functionName = name
			}
		}

		// Log the exception
		logException(category, message, 0, functionName, stackTrace)
		return nil, nil
	}

}
