package text

type Scanner interface {
	Scan() bool
	Text() string
	Err() error
}

// Create a Scanner that reads lines up to
// the first empty line, which is skipped.
func NewSectionScanner(s Scanner) Scanner {
	return &sectionScanner{Scanner: s}
}

type sectionScanner struct {
	Scanner
	text string
}

func (s *sectionScanner) Scan() (ok bool) {
	ok = s.Scanner.Scan()
	if !ok {
		return
	}
	s.text = s.Scanner.Text()
	if s.text == "" {
		ok = false
	}
	return
}

func (s *sectionScanner) Text() string {
	return s.text
}
