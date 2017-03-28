package text

type Scanner interface {
	Scan() bool
	Text() string
	Err() error
}

// Create a Scanner that reads lines up to
// the first empty line, which is skipped.
func NewSectionScanner(s Scanner) *SectionScanner {
	return &SectionScanner{Scanner: s, NumSepLines: 1}
}

type SectionScanner struct {
	Scanner
	text        string
	NumSepLines int
	n           int
}

func (s *SectionScanner) Scan() (ok bool) {
	ok = s.Scanner.Scan()
	if !ok {
		return false
	}
	s.text = s.Scanner.Text()
	if s.text == "" {
		s.n++
		if s.n == s.NumSepLines {
			return false
		}
	} else {
		s.n = 0
	}
	return true
}

func (s *SectionScanner) Text() string {
	return s.text
}

type multiScanner struct {
	c    chan scanLine
	line scanLine
}
type scanLine struct {
	text string
	err  error
}

func MultiScanner(scanners ...Scanner) Scanner {
	m := new(multiScanner)
	m.c = make(chan scanLine, 8)
	for i := range scanners {
		s := scanners[i]
		go func() {
			for s.Scan() {
				m.c <- scanLine{text: s.Text()}
			}
			m.c <- scanLine{err: s.Err()}
		}()
	}
	return m
}

func (m *multiScanner) Scan() (ok bool) {
	m.line = <-m.c
	ok = m.line.err == nil
	return
}

func (m *multiScanner) Text() string {
	return m.line.text
}

func (m *multiScanner) Err() error {
	return m.line.err
}
