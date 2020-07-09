package stringutil

// FindOpeningBracket performs a backward search on the string
// argument for a matching bracket, provided that closingBracketIndex
// points to the closing bracket.
// The function recognizes nested brackets; it returns -1 if no matching
// opening bracket could be found.
func FindOpeningBracket(s string, openingBracket byte, closingBracketIndex int) int {
	openCnt := 1
	i := closingBracketIndex
	if i >= len(s) {
		return -1
	}
	closingBracket := s[i]
	for i--; i >= 0; i-- {
		if s[i] == closingBracket {
			openCnt++
		} else if s[i] == openingBracket {
			openCnt--
			if openCnt == 0 {
				return i
			}
		}
	}
	return -1
}
