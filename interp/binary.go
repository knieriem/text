package interp

import (
	"encoding/binary"
	"strconv"
)

var binCmds = Cmd{
	Help: "binary encoding commands",
	Map: CmdMap{
		"be": makeBinEncoding(binary.BigEndian, "big endian"),
		"le": makeBinEncoding(binary.LittleEndian, "little endian"),
	},
}

func makeBinEncoding(bo binary.ByteOrder, name string) *Cmd {
	return &Cmd{
		Help: "binary encoding commands (" + name + ")",
		Map: CmdMap{
			"uint16":    binaryDecodeCmd(bo.Uint16, 2, "uint16"),
			"uint32":    binaryDecodeCmd(bo.Uint32, 4, "uint32"),
			"uint64":    binaryDecodeCmd(bo.Uint64, 8, "uint64"),
			"putuint16": binaryEncodeCmd(bo.PutUint16, 16, 2, "uint16"),
			"putuint32": binaryEncodeCmd(bo.PutUint32, 32, 4, "uint32"),
			"putuint64": binaryEncodeCmd(bo.PutUint64, 64, 8, "uint64"),
		},
	}
}

func binaryDecodeCmd[T uint16 | uint32 | uint64](get func([]byte) T, n int, name string) *Cmd {
	return &Cmd{
		Help: "convert byte slice into " + name + " value",
		Arg:  []string{"BYTES", "..."},
		Fn: func(ctx Context, arg []string) error {
			if len(arg[1:]) != n {
				return ErrWrongNArg
			}
			b, err := Argbytes(arg[1:])
			if err != nil {
				return err
			}
			u := T(get(b))
			ctx.Printf("%v", u)
			return nil
		},
	}
}

func binaryEncodeCmd[T uint16 | uint32 | uint64](put func([]byte, T), w, n int, name string) *Cmd {
	return &Cmd{
		Help: "convert " + name + " value to a byte slice",
		Arg:  []string{"VALUE"},
		Fn: func(ctx Context, arg []string) error {
			b := make([]byte, n)
			u, err := strconv.ParseUint(arg[1], 0, w)
			if err != nil {
				i, err := strconv.ParseInt(arg[1], 0, w)
				if err != nil {
					return err
				}
				u = uint64(i)
			}
			put(b, T(u))
			ctx.Printf("% #02x", b)
			return nil
		},
	}
}
