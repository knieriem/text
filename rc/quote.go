package rc

import (
	"strings"
	"unicode/utf8"
)

func NeedsQuote(r rune) bool {
	if r <= ' ' {
		return true
	}
	if r >= utf8.RuneSelf {
		return false
	}
	if strings.IndexByte("`^#*[]=|\\?${}()'<>&;", byte(r)) != -1 {
		return true
	}
	return false
}

func Quote(s string) string {
	return quote(s, "")
}

func QuoteCmd(s string) string {
	return quote(s, "=")
}

func quote(s, unquoted string) string {
	quotePart := false
	q := ""
	addPart := func(part string) {
		if quotePart {
			q += `'` + strings.Replace(part, `'`, `''`, -1) + `'`
			quotePart = false
		} else {
			q += part
		}

	}
	i0 := 0
	for i, r := range s {
		if unquoted != "" {
			if r >= utf8.RuneSelf {
				continue
			}
			if strings.IndexByte(unquoted, byte(r)) != -1 {
				if i > i0 {
					addPart(s[i0:i])
				}
				q += string(r)
				i0 = i + 1
				continue
			}
		}
		if NeedsQuote(r) {
			quotePart = true
		}
	}
	if len(s) > i0 {
		addPart(s[i0:])
	}
	return q
}

func Join(list []string) (js string) {
	if len(list) == 0 {
		return
	}
	for _, s := range list {
		js += " " + Quote(s)
	}
	return
}

func JoinCmd(list []string) (js string) {
	if len(list) == 0 {
		return
	}
	for _, s := range list {
		js += " " + QuoteCmd(s)
	}
	return
}
