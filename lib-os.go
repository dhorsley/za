//+build !test

package main

import (
	"errors"
	"os"
)

func buildOsLib() {

	// os level

	features["os"] = Feature{version: 1, category: "os"}
	categories["os"] = []string{"env", "get_env", "set_env"}

	slhelp["env"] = LibHelp{in: "", out: "string", action: "Return all available environmental variables."}
	stdlib["env"] = func(args ...interface{}) (ret interface{}, err error) {
		return os.Environ(), err // testing
	}

	// get environmental variable. arg should *usually* be in upper-case.
	slhelp["get_env"] = LibHelp{in: "key_name", out: "string", action: "Return the value of the environmental variable [#i1]key_name[#i0]."}
	stdlib["get_env"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args)!=1 { return "",errors.New("Bad args (count) in get_env()") }
        return os.Getenv(args[0].(string)), err
	}

	// set environmental variable.
	slhelp["set_env"] = LibHelp{in: "key_name,value_string", out: "", action: "Set the value of the environmental variable [#i1]key_name[#i0]."}
	stdlib["set_env"] = func(args ...interface{}) (ret interface{}, err error) {
		if len(args) != 2 {
			return nil, errors.New("Error: bad arguments to set_env()")
		}
		key := args[0].(string)
		val := args[1].(string)
		return os.Setenv(key, val), err
	}

}
