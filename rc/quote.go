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
	var q strings.Builder
	addPart := func(part string) {
		if quotePart {
			q.WriteString(`'` + strings.Replace(part, `'`, `''`, -1) + `'`)
			quotePart = false
		} else {
			q.WriteString(part)
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
				q.WriteString(string(r))
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
	return q.String()
}

func Join(list []string) string {
	if len(list) == 0 {
		return ""
	}
	var js strings.Builder
	for _, s := range list {
		js.WriteString(" " + Quote(s))
	}
	return js.String()[1:]
}

func JoinCmd(list []string) string {
	if len(list) == 0 {
		return ""
	}
	var js strings.Builder
	for _, s := range list {
		js.WriteString(" " + QuoteCmd(s))
	}
	return js.String()[1:]
}
