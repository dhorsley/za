//go:build !test
// +build !test

package main

import (
	"errors"
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

}
