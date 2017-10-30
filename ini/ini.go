package ini

import (
	"bufio"
	"flag"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/knieriem/vfsutil"
	"golang.org/x/tools/godoc/vfs"

	"github.com/knieriem/text/line"
	"github.com/knieriem/text/tidata"
)

var ns = vfs.NameSpace{}

type File struct {
	Name       string
	short      string
	overridden string
	ns         vfs.NameSpace
	Using      string
}

func NewFile(name, short, option string) (f *File) {
	f = new(File)
	f.Name = name
	f.short = short
	if option != "" {
		flag.StringVar(&f.overridden, option, "", "specify an alternative configuration file")
	}
	return
}

func BindFS(fs vfs.FileSystem) {
	ns.Bind("/", fs, "/", vfs.BindBefore)
}

func BindHomeLib() {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		return
	}
	lib := filepath.Join(u.HomeDir, "lib")
	ns.Bind("/", vfsutil.LabeledOS(lib, "$home/lib"), "/", vfs.BindBefore)
}

func BindHomeLibDir(subDir string) {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		return
	}
	lib := filepath.Join(u.HomeDir, "lib", subDir)
	ns.Bind("/", vfsutil.LabeledOS(lib, "$home/lib/"+subDir), "/", vfs.BindBefore)
}

func LookupFiles(dir, ext string) ([]File, error) {
	var f []File

	list, err := ns.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, fi := range list {
		if fi.IsDir() {
			continue
		}
		if path.Ext(fi.Name()) == ext {
			f = append(f, File{Name: path.Join(dir, fi.Name())})
		}
	}
	return f, nil
}

func (f *File) Parse(conf interface{}) (err error) {
	var r io.ReadCloser

	name := f.Name
	ini := f.short
	using := "no " + ini + " file"
	defer func() {
		f.Using = "using " + using
	}()
	if f.overridden != "" {
		name = f.overridden
		r, err = os.Open(f.overridden)
		if err != nil {
			return nil
		}
		using = ini + " from cmd line"
	} else {
		r, err = ns.Open(name)
		if err != nil {
			return nil
		}
		using = ini
		if lb, ok := r.(vfsutil.Label); ok {
			s := lb.Label()
			if s == "builtin" {
				using = "builtin " + ini
			} else {
				using += " from " + s
			}
		}
	}
	err = Parse(r, conf)
	if err != nil {
		err = line.ErrInsertFilename(err, name)
	}
	r.Close()
	return err
}

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
		if strings.HasSuffix(name, "Id") {
			name = name[:len(name)-1] + "D"
		} else if strings.HasSuffix(name, "Url") {
			name = name[:len(name)-2] + "RL"
		}
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
