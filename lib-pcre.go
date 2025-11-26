//go:build !test && !netgo && !windows

package main

import (
	"github.com/GRbit/go-pcre"
)

func buildRegexLib() {

	features["pcre"] = Feature{version: 1, category: "text"}
	categories["pcre"] = []string{"reg_match", "reg_filter", "reg_replace"}

	slhelp["reg_replace"] = LibHelp{in: "var,regex,replacement[,int_flags]", out: "string", action: "Replaces matches found in [#i1]var[#i0] with [#i1]regex[#i0] to [#i1]replacement[#i0]."}
	stdlib["reg_replace"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("reg_replace", args, 2,
			"3", "string", "string", "string",
			"4", "string", "string", "string", "int"); !ok {
			return nil, err
		}

		src := args[0].(string)
		regex := args[1].(string)
		repl := args[2].(string)
		flags := 0
		if len(args) == 4 {
			flags = args[3].(int)
		}

		var re pcre.Regexp
		re = pcre.MustCompileParseJIT(regex, pcre.STUDY_JIT_COMPILE)
		// pf("(rr) about to use src : %s\n",src)
		// pf("(rr) about to use repl: %s\n",repl)
		s := re.ReplaceAllString(src, repl, flags)
		return string(s), nil
	}

	slhelp["reg_match"] = LibHelp{in: "string,regex", out: "bool", action: "Does [#i1]string[#i0] contain a match for regular expression [#i1]regex[#i0]?"}
	stdlib["reg_match"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("reg_match", args, 1, "2", "string", "string"); !ok {
			return "", err
		}
		b := args[0].(string)
		if args[1].(string) == "" {
			return true, nil
		}
		m := pcre.MustCompileParseJIT(args[1].(string), pcre.STUDY_JIT_COMPILE).NewMatcherString(b, 0)
		n := 0
		for f := m.Matches; f; f = m.MatchStringWFlags(b, 0) {
			n++
			b = b[m.Index()[1]:]
		}
		return n > 0, nil
	}

	slhelp["reg_filter"] = LibHelp{in: "string,regex[,count]", out: "[][start_pos,end_pos]", action: "Returns a list of start and end positions where matches were encountered."}
	stdlib["reg_filter"] = func(ns string, evalfs uint32, ident *[]Variable, args ...any) (ret any, err error) {
		if ok, err := expect_args("reg_filter", args, 1,
			"2", "string", "string"); !ok {
			return "", err
		}
		re := pcre.MustCompileParseJIT(args[1].(string), pcre.STUDY_JIT_COMPILE)
		return re.FindAllIndex([]byte(args[0].(string)), 0), nil
	}

}
