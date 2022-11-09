package ini

import (
	"bufio"
	"flag"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/knieriem/fsutil"

	"github.com/knieriem/text/line"
	"github.com/knieriem/text/tidata"
)

var ns = fsutil.NameSpace{}

type File struct {
	Name       string
	short      string
	overridden string
	Using      string
	Label      string
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

func BindFS(fsys fs.FS) {
	ns.Bind(".", fsys, fsutil.BindBefore())
}

func BindOS(path, label string) {
	ns.Bind(".", os.DirFS(path), withLabel(label), fsutil.BindBefore())
}

func BindHomeLib() {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		return
	}
	lib := filepath.Join(u.HomeDir, "lib")
	ns.Bind(".", os.DirFS(lib), withLabel("$home/lib"), fsutil.BindBefore())
}

func BindHomeLibDir(subDir string) {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		return
	}
	lib := filepath.Join(u.HomeDir, "lib", subDir)
	ns.Bind(".", os.DirFS(lib), withLabel("$home/lib/"+subDir), fsutil.BindBefore())
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
	label := ""
	using := "no " + ini + " file"
	defer func() {
		f.Label = label
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
		using = name
		var inf fsAnnotations
		if inf.from(r) {
			name = inf.absPath(name)
			label = inf.label
			if inf.isBuiltin() {
				using = "builtin " + ini
			} else {
				using += " from " + label
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

// A DecodeFn parses, and decodes a configuration file and stores it
// in the value pointed to by v.
type DecodeFn func(v interface{}) error

// A WalkFn is called for each part that is found by WalkParts. It
// should call decode to parse the part's contents into the provided
// variable. WalkFn should return the error returned by decode,
// or nil.
type WalkFn func(partName string, decode DecodeFn) error

// WalkParts looks for a file or directory with the given name -- with
// or without the extension -- within the configured namespace. If it
// finds a directory, each file within that directory is considered a
// part, and walkFn is called for it. If WalkParts finds a single
// file, walkFn is called once instead, the single file treated as one
// part in this case.
// This way WalkParts assists parsing configuration, whether it is
// contained in one part, or split over multiple project files within
// a directory. How exactly the parts are handled, is left to the
// caller, who specifies walkFn.
func WalkParts(name string, walkFn WalkFn) (label string, err error) {
	var inf fsAnnotations
	ext := path.Ext(name)
	stem := name[:len(name)-len(ext)]
	fi, err := fs.Stat(ns, name)
	if err != nil {
		// name does not exist, lookup stem instead
		fi1, err1 := fs.Stat(ns, stem)
		if err1 != nil || !fi1.IsDir() {
			return "", err
		}
		inf.from(fi1)
		name = stem
		fi = fi1
	} else if inf.from(fi) && inf.isBuiltin() {
		// found a builtin configuration, try to lookup
		// a non-builtin stem config
		fi1, err := fs.Stat(ns, stem)
		if err == nil && fi1.IsDir() {
			if inf.from(fi1) && !inf.isBuiltin() {
				name = stem
				fi = fi1
			}
		}
	}

	if fi.IsDir() {
		err = parseDir(name, ext, walkFn, &inf)
	} else {
		err = parsePart(name, walkFn, &inf)
	}
	return inf.label, err
}

func parseDir(dirname, ext string, walkFn WalkFn, inf *fsAnnotations) error {
	list, err := ns.ReadDir(dirname)
	if err != nil {
		return err
	}
	for _, d := range list {
		if d.IsDir() {
			continue
		}
		name := d.Name()
		if path.Ext(name) != ext {
			continue
		}
		path := path.Join(dirname, name)
		err := parsePart(path, walkFn, inf)
		if err != nil {
			return err
		}
	}
	return nil
}

func parsePart(name string, walkFn WalkFn, inf *fsAnnotations) error {
	err := walkFn(path.Base(name), func(data interface{}) error {
		f, err := ns.Open(name)
		if err != nil {
			return err
		}
		return Parse(f, data)
	})
	if err != nil {
		err = line.ErrInsertFilename(err, inf.absPath(name))
	}
	return err
}

// ParseFile parses a single configuration file that is found in the
// configured namespace under the given name. A label referring to the
// file system, where the file was found, is returned.
func ParseFile(name string, conf interface{}) (fsLabel string, err error) {
	f := NewFile(name, "", "")
	err = f.Parse(conf)
	return f.Label, err
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

func withLabel(val string) fsutil.BindOption {
	return fsutil.WithValue(fsutil.LabelKey, val)
}

type fsAnnotations struct {
	label  string
	fsRoot string
}

func (a *fsAnnotations) from(item interface{}) bool {
	it, ok := item.(fsutil.Item)
	if !ok {
		return false
	}
	fsys := it.FS()
	label, ok := fsutil.Value(fsys, fsutil.LabelKey).(string)
	if !ok {
		return false
	}
	a.label = label
	a.fsRoot, _ = fsutil.Value(fsys, fsutil.RootOSDirKey).(string)
	return true
}

func (a *fsAnnotations) isBuiltin() bool {
	return a.label == "builtin"
}

func (a *fsAnnotations) absPath(name string) string {
	if a.fsRoot != "" {
		return filepath.Join(a.fsRoot, name)
	}
	if a.label == "" {
		return name
	}
	return a.label + ":" + name
}
