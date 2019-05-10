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

func BindFS(fs vfs.FileSystem) {
	ns.Bind("/", fs, "/", vfs.BindBefore)
}

func BindOS(path, label string) {
	ns.Bind("/", vfsutil.LabeledOS(path, label), "/", vfs.BindBefore)
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
		using = ini
		if lb, ok := r.(vfsutil.Label); ok {
			label = lb.Label()
			if label == "builtin" {
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
	fsRoot := ""
	ext := filepath.Ext(name)
	stem := name[:len(name)-len(ext)]
	fi, err := ns.Stat(name)
	if err != nil {
		// name does not exist, lookup stem instead
		fi1, err1 := ns.Stat(stem)
		if err1 != nil || !fi1.IsDir() {
			return "", err
		}
		if fs, ok := fi1.(vfsutil.FSInfo); ok {
			label = fs.Label()
			fsRoot = fs.Root()
		}
		name = stem
		fi = fi1
	} else if fs, ok := fi.(vfsutil.FSInfo); ok {
		label = fs.Label()
		fsRoot = fs.Root()
		if label == "builtin" {
			// found a builtin configuration, try to lookup
			// a non-builtin stem config
			fi1, err := ns.Stat(stem)
			if err == nil && fi1.IsDir() {
				if fs, ok := fi1.(vfsutil.FSInfo); ok {
					if l1 := fs.Label(); l1 != "builtin" {
						name = stem
						fi = fi1
					}
				}
			}
		}
	}

	if fi.IsDir() {
		err = parseDir(fsRoot, name, ext, walkFn)
	} else {
		err = parsePart(fsRoot, name, walkFn)
	}
	return label, err
}

func parseDir(fsRoot, dirname, ext string, walkFn WalkFn) error {
	list, err := ns.ReadDir(dirname)
	if err != nil {
		return err
	}
	for _, fi := range list {
		if fi.IsDir() {
			continue
		}
		name := fi.Name()
		if path.Ext(name) != ext {
			continue
		}
		path := filepath.Join(dirname, name)
		err := parsePart(fsRoot, path, walkFn)
		if err != nil {
			return err
		}
	}
	return nil
}

func parsePart(fsRoot, name string, walkFn WalkFn) error {
	err := walkFn(filepath.Base(name), func(data interface{}) error {
		f, err := ns.Open(name)
		if err != nil {
			return err
		}
		return Parse(f, data)
	})
	if err != nil {
		err = line.ErrInsertFilename(err, filepath.Join(fsRoot, name))
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
