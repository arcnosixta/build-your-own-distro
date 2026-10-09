package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	cpioMagic = "070701"
	modeReg   = 0100000
	modeDir   = 0040000
	modeLnk   = 0120000
)

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func entryMode(name string, info os.FileInfo) uint32 {
	switch {
	case info.IsDir():
		return modeDir | 0o755
	case info.Mode()&os.ModeSymlink != 0:
		return modeLnk | 0o777
	}
	perm := uint32(info.Mode().Perm())
	switch {
	case perm&0o111 != 0:
		return modeReg | 0o755
	case isExecPath(name):
		return modeReg | 0o755
	case perm == 0 || runtime.GOOS == "windows":
		return modeReg | 0o644
	default:
		return modeReg | perm
	}
}

func isExecPath(name string) bool {
	for _, d := range []string{"./bin/", "./sbin/", "./usr/bin/", "./usr/sbin/", "./usr/local/bin/", "./usr/local/sbin/"} {
		if strings.HasPrefix(name, d) {
			return true
		}
	}
	return false
}

func packInitramfs(root, out string) (int, int64, error) {
	f, err := os.Create(out)
	if err != nil {
		return 0, 0, err
	}

	gz := gzip.NewWriter(f)
	cw := &countWriter{w: gz}
	ino := uint32(0)
	files := 0

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if name != "." {
			name = "./" + name
		}
		ino++
		mtime := info.ModTime().Unix()

		switch {
		case info.IsDir():
			if err := writeEntry(cw, name, entryMode(name, info), 0, mtime, ino, nil); err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := writeEntry(cw, name, entryMode(name, info), int64(len(target)), mtime, ino, asReader(target)); err != nil {
				return err
			}
		default:
			src, err := os.Open(path)
			if err != nil {
				return err
			}
			werr := writeEntry(cw, name, entryMode(name, info), info.Size(), mtime, ino, src)
			src.Close()
			if werr != nil {
				return werr
			}
		}
		files++
		return nil
	})
	if err != nil {
		f.Close()
		return 0, 0, err
	}

	ino++
	if err := writeEntry(cw, "TRAILER!!!", 0, 0, 0, ino, nil); err != nil {
		f.Close()
		return 0, 0, err
	}
	if pad := (512 - cw.n%512) % 512; pad > 0 {
		if _, err := cw.Write(make([]byte, pad)); err != nil {
			f.Close()
			return 0, 0, err
		}
	}

	if err := gz.Close(); err != nil {
		f.Close()
		return 0, 0, err
	}
	if err := f.Close(); err != nil {
		return 0, 0, err
	}

	st, err := os.Stat(out)
	if err != nil {
		return files, 0, err
	}
	return files, st.Size(), nil
}

type stringReader struct {
	s string
	i int
}

func asReader(s string) io.Reader { return &stringReader{s: s} }

func (r *stringReader) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

func writeEntry(w io.Writer, name string, mode uint32, size int64, mtime int64, ino uint32, data io.Reader) error {
	nameb := name + "\x00"
	header := fmt.Sprintf("%s%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X",
		cpioMagic, ino, mode, 0, 0, 1, mtime, size, 0, 0, 0, 0, len(nameb), 0)
	if len(header) != 110 {
		return fmt.Errorf("cpio header must be 110 bytes, got %d", len(header))
	}
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	if _, err := io.WriteString(w, nameb); err != nil {
		return err
	}
	if pad := (4 - (110+len(nameb))%4) % 4; pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}
	if data != nil && size > 0 {
		if _, err := io.CopyN(w, data, size); err != nil {
			return err
		}
	}
	if pad := (4 - size%4) % 4; pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}
	return nil
}
