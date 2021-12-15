package main

import (
	"unicode/utf8"
    "regexp"
    str "strings"
)


// line grep from string
func lgrep(s string,reg string) string {
    list:=str.Split(s,"\n")
    var ns str.Builder
    for _,v:=range list {
        if m,_:=regexp.MatchString(reg,v); m {
            ns.WriteString(v+"\n")
        }
    }
    repl:=ns.String()
    if len(repl)>0 {
        if repl[len(repl)-1]=='\n' {
            repl=repl[:len(repl)-1]
        }
    }
    return repl
}

func lcut(s string,pos int,sep string) string {
    fstr:=str.TrimSuffix(s,"\n")
    ta:=str.FieldsFunc(fstr,func(c rune) bool { return str.ContainsRune(sep,c) })
    if pos>0 && pos<=len(ta) {
        return ta[pos-1]
    }
    return ""
}

func lastCharSize(s string) int {
	_, size := utf8.DecodeLastRuneInString(s)
	return size
}

func pad(s string, just int, w int, fill string) string {

    if s=="" {
       return ""
    }

	ls := utf8.RuneCountInString(StripCC(s))
	if ls == 0 {
		return ""
	}

	switch just {

	case -1:
		// left
		return s + rep(fill,w-ls)

	case 1:
		// right
		if ls > w {
			s = string([]rune(s)[:w])
		}
		return rep(fill, int(w-utf8.RuneCountInString(s))) + s

	case 0:
		// center
		p := int(w/2) - int(ls/2)
		extra := 1
		if (w % 2) == 0 {
			extra = 0
		}
		r_remove := ls % 2
		if extra == 1 && r_remove == 1 {
			extra = 0
			r_remove = 0
		}
		return rep(fill, p+extra) + s + rep(fill, p-r_remove)

	}
	return ""
}


func sanitise(s string) string {
    var ns str.Builder
    pass:=true
    nest:=0
    for p:=0;p<len(s); p++ {
        if s[p]=='{' { nest+=1; pass=false }
        if s[p]=='}' {
            if nest!=0 {
                nest-=1
            }
            if nest==0 && pass==false {
                pass=true
                continue
            }
        }
        if pass { ns.WriteByte(s[p]) }
    }
    return ns.String()
}

func stripOuter(s string, c byte) string {
	if len(s) > 0 && s[0] == c {
        s=s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == c {
        s=s[:len(s)-1]
	}
	return s
}

func stripSingleQuotes(s string) string {
	return stripOuter(s, '\'')
}

func stripBacktickQuotes(s string) string {
	return stripOuter(s, '`')
}

func stripDoubleQuotes(s string) string {
	return stripOuter(s, '"')
}

func stripOuterQuotes(s string, maxdepth int) string {

	for ; maxdepth > 0; maxdepth-- {
		s = stripSingleQuotes(s)
		s = stripDoubleQuotes(s)
		if !(hasOuterSingleQuotes(s) || hasOuterDoubleQuotes(s)) {
			break
		}
	}
	return s
}

func hasOuterBraces(s string) bool {
	if len(s) > 0 && s[0] == '(' && s[len(s)-1] == ')' {
		return true
	}
	return false
}

func hasOuter(s string, c byte) bool {
	if len(s) > 0 && s[0] == c && s[len(s)-1] == c {
		return true
	}
	return false
}

func hasOuterBacktickQuotes(s string) bool {
	return hasOuter(s, '`')
}

func hasOuterSingleQuotes(s string) bool {
	return hasOuter(s, '\'')
}

func hasOuterDoubleQuotes(s string) bool {
	return hasOuter(s, '"')
}

