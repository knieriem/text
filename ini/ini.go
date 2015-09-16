package ini

import (
	"bufio"
	"io"
	"strings"

	"github.com/knieriem/text/tidata"
)

var MultiStringSep string

func Parse(r io.Reader, conf interface{}) (err error) {
	el, err := readTiData(r)
	if err != nil {
		return
	}

	ticonf.MultiStringSep = MultiStringSep
	err = el.Decode(conf, &ticonf)
	if err != nil {
		return
	}
	return
}

func readTiData(r io.Reader) (el *tidata.Elem, err error) {
	tr := tidata.NewReader(bufio.NewScanner(r))
	tr.CommentPrefix = "#"
	tr.CommentPrefixEscaped = `\#`
	tr.StripUtf8BOM = true
	el, err = tr.ReadAll()
	return
}

var ticonf = tidata.Config{
	MapSym: "",
	KeyToFieldName: func(key string) (name string) {
		s := strings.Title(key)
		s = replaceSpecial(s, "-", "")
		name = replaceSpecial(s, "/", "Per")
		return
	},
}

func replaceSpecial(s, old, new string) string {
	f := strings.Split(s, old)
	if len(f) == 0 {
		return s
	}
	s = f[0]
	for _, seg := range f[1:] {
		s += new + strings.Title(seg)
	}
	return s
}
