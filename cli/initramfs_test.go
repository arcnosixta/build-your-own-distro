package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWriteEntryHeaderLayout(t *testing.T) {
	var buf bytes.Buffer
	name := "./etc/passwd"
	if err := writeEntry(&buf, name, modeReg|0o644, 5, 0, 1, strings.NewReader("hello")); err != nil {
		t.Fatalf("writeEntry: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 110 {
		t.Fatalf("output too short: %d", len(b))
	}
	if magic := string(b[:6]); magic != cpioMagic {
		t.Errorf("magic = %q, want %q", magic, cpioMagic)
	}
	get := func(off int) int {
		v, err := strconv.ParseUint(string(b[off:off+8]), 16, 32)
		if err != nil {
			t.Fatalf("field at %d not hex: %q", off, string(b[off:off+8]))
		}
		return int(v)
	}
	if got := get(54); got != 5 {
		t.Errorf("filesize = %d, want 5", got)
	}
	if got := get(94); got != len(name)+1 {
		t.Errorf("namesize = %d, want %d", got, len(name)+1)
	}
	// name(12) pads header to 124, then 5 bytes of data pads to 132.
	if buf.Len() != 132 {
		t.Errorf("total length = %d, want 132", buf.Len())
	}
}

func TestEntryModeMarksExecutables(t *testing.T) {
	f := filepath.Join(t.TempDir(), "prog")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := entryMode("./bin/prog", info); got != modeReg|0o755 {
		t.Errorf("entryMode(/bin) = %o, want %o", got, modeReg|0o755)
	}
	if got := entryMode("./etc/config", info); got != modeReg|0o644 {
		t.Errorf("entryMode(/etc) = %o, want %o", got, modeReg|0o644)
	}
}

func parseCpio(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	i := 0
	for {
		if i+110 > len(raw) {
			t.Fatalf("truncated cpio header at %d", i)
		}
		hdr := string(raw[i : i+110])
		if hdr[:6] != cpioMagic {
			t.Fatalf("bad cpio magic %q at %d", hdr[:6], i)
		}
		field := func(off int) int {
			v, err := strconv.ParseUint(hdr[off:off+8], 16, 32)
			if err != nil {
				t.Fatalf("bad cpio field %q", hdr[off:off+8])
			}
			return int(v)
		}
		size := field(54)
		nameSize := field(94)
		i += 110
		name := string(raw[i : i+nameSize-1])
		i += nameSize
		i = (i + 3) &^ 3
		body := raw[i : i+size]
		i += size
		i = (i + 3) &^ 3
		if name == "TRAILER!!!" {
			break
		}
		out[name] = append([]byte(nil), body...)
	}
	return out
}

func TestPackInitramfsRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "usr", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := "hello initramfs\n"
	if err := os.WriteFile(filepath.Join(root, "usr", "bin", "hello"), []byte(want), 0o755); err != nil {
		t.Fatal(err)
	}
	hasLink := os.Symlink("/bin/sh", filepath.Join(root, "usr", "bin", "sh")) == nil

	out := filepath.Join(t.TempDir(), "initramfs.cpio.gz")
	files, size, err := packInitramfs(root, out)
	if err != nil {
		t.Fatalf("packInitramfs: %v", err)
	}
	if files == 0 || size == 0 {
		t.Fatalf("packInitramfs reported files=%d size=%d", files, size)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}

	entries := parseCpio(t, raw)
	if got := string(entries["./usr/bin/hello"]); got != want {
		t.Errorf("hello content = %q, want %q", got, want)
	}
	if hasLink {
		if got := string(entries["./usr/bin/sh"]); got != "/bin/sh" {
			t.Errorf("symlink target = %q, want %q", got, "/bin/sh")
		}
	}
	if _, ok := entries["."]; !ok {
		t.Error("missing root directory entry")
	}
}
