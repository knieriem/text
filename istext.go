package text

import (
	"unicode/utf8"
)

// IsText returns true if bytes in b form valid UTF-8 characters, and
// if b doesn't contain any unprintable ASCII or Unicode characters.
func IsText(b []byte, extraChars []rune) bool {
	for len(b) > 0 && utf8.FullRune(b) {
		r, size := utf8.DecodeRune(b)
		if size == 1 && r == utf8.RuneError {
			// decoding error
			return false
		}
		if 0x7F <= r && r <= 0x9F {
			return false
		}
		if r < ' ' {
		S:
			switch r {
			case '\n', '\r', '\t', '\f':
				// okay
			default:
				for _, c := range extraChars {
					if r == c {
						break S
					}
				}
				// binary garbage
				return false
			}
		}
		b = b[size:]
	}
	return true
}
