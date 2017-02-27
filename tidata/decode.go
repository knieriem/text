package tidata

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/knieriem/text/line"
	"github.com/knieriem/text/rc"
)

// An UnmarshalTypeError describes a tidata value that was
// not appropriate for a value of a specific Go type.
type UnmarshalTypeError struct {
	Value string       // description of tidata value - "bool", "array", "number -5"
	Type  reflect.Type // type of Go value it could not be assigned to
}

func (e *UnmarshalTypeError) Error() string {
	return "tidata: cannot unmarshal " + e.Value + " into Go value of type " + e.Type.String()
}

type Config struct {
	Sep            string // a string separating key and value, e.g. ":"
	MapSym         string
	KeyToFieldName func(string) string
	MultiStringSep string
}

var dfltConfig = Config{
	Sep:    ":",
	MapSym: ":",
	KeyToFieldName: func(key string) (field string) {
		field = strings.Replace(key, "-", "", -1)
		return
	},
	MultiStringSep: "\n",
}

type decoder struct {
	*Config

	cur struct {
		field string
		line  int
	}
	errList line.ErrorList

	deferredWork []deferred
}

type Deferred interface {
	DeferredWork(arg interface{}) error
}

type DeferredWorkRunner interface {
	RunDeferredWork(func(arg interface{}) error) error
}

type deferred struct {
	fn    func(interface{}) error
	line  int
	field string
}

type Error struct {
	Err  error
	Key  string
	line int
}

func (e *Error) Line() int {
	return e.line
}

func (e *Error) Error() string {
	return fmt.Sprintf("tidata: %s: %s", e.Key, e.Err.Error())
}

func (d *decoder) saveError(err error) {
	e := &Error{
		line: d.cur.line,
		Err:  err,
		Key:  d.cur.field,
	}
	d.errList.Add(e)
}

func (e Elem) Decode(i interface{}, c *Config) (err error) {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		err = errors.New("argument is not a pointer to an object")
		return
	}
	v = v.Elem()

	d := new(decoder)
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = &Error{
				line: d.cur.line,
				Err:  r.(error),
				Key:  d.cur.field,
			}
		}
	}()

	if c == nil {
		c = &dfltConfig
	}
	d.Config = c
	d.decodeItem(v, e)
	if d.errList.List != nil {
		err = &d.errList
	}
	return
}

func (d *decoder) deriveKey(el Elem) (key string, err error) {
	k := el.Key()
	if k == "" {
		if el.Text == "" {
			err = errors.New("<tab> at beginning of empty line")
		} else {
			err = errors.New("empty key")
		}
		return
	}
	if d.Sep != "" {
		if !strings.HasSuffix(k, d.Sep) {
			err = errors.New("missing '" + d.Sep + "' in key")
			return
		}
		k = k[:len(k)-len(d.Sep)]
	}
	key = k
	if f := d.KeyToFieldName; f != nil {
		key = f(k)
	}
	return
}

type StructPreprocessor interface {
	Preprocess(*Elem) error
}

type Postprocessor interface {
	Postprocess() error
}

func (d *decoder) decodeStruct(dest reflect.Value, src Elem) {
	var key string
	var err error
	var anyIndex int
	var seenMap reflect.Value

	d.cur.line = src.LineNum

	t := dest.Type()
	if f := dest.FieldByName("SrcLineNum"); f.IsValid() {
		f.SetInt(int64(d.cur.line))
	}
	if f := dest.FieldByName("TidataElem"); f.IsValid() {
		f.Set(reflect.ValueOf(&src))
	}
	if f := dest.FieldByName("TidataSeen"); f.IsValid() {
		seenMap = f
	}
	if u, ok := dest.Addr().Interface().(Unmarshaler); ok {
		err := u.UnmarshalTidata(src)
		if err != nil {
			d.saveError(err)
		}
		return
	}

	d.cur.field = t.String()
	if p, ok := dest.Addr().Interface().(StructPreprocessor); ok {
		err = p.Preprocess(&src)
		if err != nil {
			d.saveError(err)
		}
	} else {
		/* look into Value() if it contains short versions of fields */
		v := src.Value()
		var pfx []Elem
		for _, x := range rc.Tokenize(v) {
			eq := strings.Index(x, "=")
			el := Elem{LineNum: d.cur.line}
			if eq != -1 {
				el.Text = x[:eq] + d.Sep + "\t" + x[eq+1:]
			} else {
				el.Text = x + d.Sep + "\ttrue"
			}
			pfx = append(pfx, el)
		}
		if pfx != nil {
			src.Children = append(pfx, src.Children...)
		}
	}

	anyIndex = -1
	for i, n := 0, t.NumField(); i < n; i++ {
		f := t.Field(i)
		if k := f.Type.Kind(); k == reflect.Slice || k == reflect.Map {
			tag := f.Tag.Get("tidata")
			if tag == "any" {
				anyIndex = i
				break
			}
		}
	}

	seenCombined := map[string]bool{}
	seen := map[string]bool{}
	for i := range src.Children {
		el := src.Children[i]
		d.cur.line = el.LineNum
		d.cur.field = el.Key()
		key, err = d.deriveKey(el)
		if err != nil {
			d.saveError(err)
			return
		}
		if seenCombined[key] {
			continue
		}
		if seen[key] {
			d.saveError(errors.New("field defined more than once"))
			continue
		}

		if f, ok := t.FieldByName(key); !ok {
			if anyIndex == -1 {
				d.saveError(errors.New("field does not exist"))
			} else {
				d.decodeItem(dest.Field(anyIndex), Elem{LineNum: el.LineNum, Children: src.Children[i:]})
				break
			}
		} else {
			v := dest.FieldByIndex(f.Index)
			tag := f.Tag.Get("tidata")
			if tag == "combine" {
				if v.Kind() == reflect.Slice {
					d.collectItems(v, key, src.Children[i:])
					seenCombined[key] = true
					d.postProcess(v, el)
					continue
				}
			}
			d.decodeItem(v, el)
			seen[key] = true
		}
	}
	if seenMap.IsValid() {
		seenMap.Set(reflect.ValueOf(seen))
	}

	if r, ok := dest.Addr().Interface().(DeferredWorkRunner); ok {
		for _, w := range d.deferredWork {
			err = r.RunDeferredWork(w.fn)
			if err != nil {
				e := &Error{
					line: w.line,
					Err:  err,
					Key:  w.field,
				}
				d.errList.Add(e)
			}
		}
		d.deferredWork = nil
	}
	return
}

func (d *decoder) postProcess(v reflect.Value, src Elem) {
	if p, ok := v.Addr().Interface().(Postprocessor); ok {
		d.cur.field = src.Key()
		d.cur.line = src.LineNum
		err := p.Postprocess()
		if err != nil {
			d.saveError(err)
		}
	}

}

func (d *decoder) collectItems(v reflect.Value, keyWant string, tail []Elem) {
	var found []Elem
	for _, el := range tail {
		key, err := d.deriveKey(el)
		if err != nil {
			return
		}
		if key == keyWant {
			found = append(found, el)
		}
	}
	mkslice(v, len(found))
	for i, el := range found {
		d.decodeItem(v.Index(i), el)
	}
}

func mkslice(v reflect.Value, n int) {
	if v.Cap() >= n {
		v.Set(v.Slice(0, n))
		return
	}
	v.Set(reflect.MakeSlice(v.Type(), n, n))
	return
}

type Unmarshaler interface {
	UnmarshalTidata(Elem) error
}

func (d *decoder) decodeItem(v reflect.Value, el Elem) {
	d.cur.line = el.LineNum

	field := d.cur.field
	defer func() {
		if p, ok := v.Addr().Interface().(Deferred); ok {
			d.deferredWork = append(d.deferredWork, deferred{fn: p.DeferredWork, line: el.LineNum, field: field})
		}
	}()

	if u, ok := v.Addr().Interface().(Unmarshaler); ok {
		err := u.UnmarshalTidata(el)
		if err != nil {
			d.saveError(err)
		}
		return
	}

retry:
	switch v.Kind() {
	case reflect.Ptr:
		if v.Type() == reflect.TypeOf(&el) {
			v.Set(reflect.ValueOf(&el))
		} else {
			vObj := reflect.New(v.Type().Elem())
			v.Set(vObj)
			v = vObj.Elem()
			goto retry
		}
	case reflect.Struct:
		d.decodeStruct(v, el)
	case reflect.Slice:
		sl := reflect.Zero(v.Type())
		if n := len(el.Children); n > 0 {
			sl = reflect.MakeSlice(v.Type(), n, n)
			switch sl.Index(0).Kind() {
			case reflect.Struct:
				for i := 0; i < n; i++ {
					d.decodeStruct(sl.Index(i), el.Children[i])
				}
			default:
				for i := 0; i < n; i++ {
					c := el.Children[i]
					d.decodeItem(sl.Index(i), Elem{Text: ".\t" + c.Text, Children: c.Children})
				}
			}
		} else if s := el.Value(); s != "" {
			list := rc.Tokenize(s)
			if n = len(list); n > 0 {
				sl = reflect.MakeSlice(v.Type(), n, n)
				for i := 0; i < n; i++ {
					d.decodeItem(sl.Index(i), Elem{Text: ".\t" + list[i]})
				}
			}
		}
		v.Set(sl)
	case reflect.Map:
		d.decodeMap(v, el)
	case reflect.String:
		val := el.Value()
		if val == "" {
			val = el.joinAllChildren("", d.MultiStringSep)
		}
		d.decodeString(v, val)
	default:
		val := el.Value()
		if val == "" {
			for i := range el.Children {
				val += el.Children[i].Text
			}
		}
		d.decodeString(v, val)
	}
	d.postProcess(v, el)
}

func (d *decoder) decodeMap(v reflect.Value, src Elem) {
	t := v.Type()
	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}

	n := len(src.Children)
	if n == 0 {
		return
	}

	key := reflect.New(t.Key()).Elem()
	val := reflect.New(t.Elem()).Elem()

	for i := 0; i < n; i++ {
		el := src.Children[i]
		if el.Text == "" {
			d.saveError(errors.New("<tab> at beginning of empty line"))
			return
		}
		kstr := el.Key()
		if kstr == el.Text && len(el.Children) == 0 && t.Elem().Kind() == reflect.Bool {
			// only allowed for map[T]bool
			d.decodeString(key, kstr)
			val.SetBool(true)
		} else {
			if d.MapSym != "" {
				if strings.HasSuffix(kstr, d.MapSym) {
					kstr = kstr[:len(kstr)-len(d.MapSym)]
				} else if j := strings.Index(el.Text, d.MapSym); j == len(kstr) {
					el.Text = kstr + "\t" + el.Text[j+len(d.MapSym)+1:]
				} else {
					d.saveError(errors.New("missing map symbol '" + d.MapSym + "' in mapping"))
					return
				}

			}
			d.cur.field = kstr
			d.decodeItem(key, Elem{LineNum: el.LineNum, Text: ".\t" + kstr})
			d.decodeItem(val, el)
		}
		v.SetMapIndex(key, val)
		key.Set(reflect.Zero(t.Key()))
		val.Set(reflect.Zero(t.Elem()))
	}
}

func (d *decoder) decodeString(v reflect.Value, s string) {
	switch v.Kind() {
	default:
		d.saveError(errors.New("data type not supported: " + v.Type().String()))

	case reflect.String:
		v.SetString(s)

	case reflect.Bool:
		switch s {
		case "true", "":
			v.SetBool(true)
		case "false":
			v.SetBool(false)
		default:
			d.saveError(&UnmarshalTypeError{"bool" + s, v.Type()})
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 0, 64)
		if err != nil || v.OverflowInt(n) {
			d.saveError(&UnmarshalTypeError{"number " + s, v.Type()})
		}
		v.SetInt(n)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(s, 0, 64)
		if err != nil || v.OverflowUint(n) {
			d.saveError(&UnmarshalTypeError{"number " + s, v.Type()})
		}
		v.SetUint(n)

	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, v.Type().Bits())
		if err != nil || v.OverflowFloat(n) {
			d.saveError(&UnmarshalTypeError{"number " + s, v.Type()})
		}
		v.SetFloat(n)
	}
	return
}
