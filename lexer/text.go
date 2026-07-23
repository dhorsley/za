package lexer

func stripOuter(s string, c byte) string {
	if len(s) > 0 && s[0] == c {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == c {
		s = s[:len(s)-1]
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
