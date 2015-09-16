package annotated

import (
	"bufio"
	"io"

	"github.com/knieriem/text/line"
)

type Chunk struct {
	Start int
	Lines []Line
}
type Line struct {
	Text   string
	Errors []line.Error
}

type File struct {
	Name string
	Chunk
	UnassociatedErrors []error
}

func ReadLines(r io.Reader) (af *File, err error) {
	af = new(File)
	s := bufio.NewScanner(r)
	for s.Scan() {
		af.Lines = append(af.Lines, Line{Text: s.Text()})
	}
	af.Start = 1
	err = s.Err()
	return
}

func (af *File) AssociateErrors(list []error) {
	unassociated := func(e error) {
		af.UnassociatedErrors = append(af.UnassociatedErrors, e)
	}
	for _, err := range list {
		if e, ok := err.(line.Error); ok {
			iLine := e.Line() - 1
			if iLine < 0 || iLine >= len(af.Lines) {
				unassociated(e)
				continue
			}
			af.Lines[iLine].Errors = append(af.Lines[iLine].Errors, e)
			continue
		}
		unassociated(err)
	}
}

func (af *File) Chunks(nContext int) (chunks []Chunk) {
	iErrPrev := -1
	i0 := 0
	for i, line := range af.Lines {
		if len(line.Errors) != 0 {
			if iErrPrev != -1 {
				if i-iErrPrev-1 > 2*nContext {
					chunks = append(chunks, Chunk{Start: af.Start + i0, Lines: af.Lines[i0 : iErrPrev+nContext+1]})
					i0 = i - nContext

				}
			}
			iErrPrev = i
		} else if iErrPrev == -1 {
			if i-i0+1 == nContext {
				i0++
			}
		}
	}
	if iErrPrev != -1 {
		end := iErrPrev + nContext + 1
		if end > len(af.Lines) {
			end = len(af.Lines)
		}
		chunks = append(chunks, Chunk{Start: af.Start + i0, Lines: af.Lines[i0:end]})
	}
	return
}
