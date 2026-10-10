package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type pkgEntry struct {
	name string
	body string
}

func writePkg(t *testing.T, path string, entries []pkgEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create pkg: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	man := `{"name":"evil","version":"1.0.0"}`
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(man)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write manifest header: %v", err)
	}
	if _, err := tw.Write([]byte(man)); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatalf("write %s header: %v", e.name, err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatalf("write %s: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("file close: %v", err)
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	bad := []string{"../escape", "../../etc/passwd", "a/../../b", "/../x"}
	for _, rel := range bad {
		if _, err := safeJoin(root, rel); err == nil {
			t.Errorf("safeJoin(%q) = nil error, want rejection", rel)
		}
	}
	good := []string{"usr/bin/hello", "etc/passwd", "opt/x", "var/lib/pkg/db.json"}
	for _, rel := range good {
		p, err := safeJoin(root, rel)
		if err != nil {
			t.Errorf("safeJoin(%q) unexpected error: %v", rel, err)
			continue
		}
		if !strings.HasPrefix(p, root) {
			t.Errorf("safeJoin(%q) = %q, want inside %q", rel, p, root)
		}
	}
}

func TestStoredPathRoundTrip(t *testing.T) {
	root := t.TempDir()
	got, err := storedPath(root, "/usr/bin/hello")
	if err != nil {
		t.Fatalf("storedPath: %v", err)
	}
	want := filepath.Join(root, "usr", "bin", "hello")
	if got != want {
		t.Errorf("storedPath = %q, want %q", got, want)
	}
	if _, err := storedPath(root, "/../../etc/passwd"); err == nil {
		t.Error("storedPath accepted an escaping path")
	}
}

func TestSaveDBAtomicRoundTrip(t *testing.T) {
	root := t.TempDir()
	db := &database{Packages: map[string]entry{"hello": {Version: "1.0.0", Files: []string{"/usr/bin/hello"}}}}
	if err := saveDB(root, db); err != nil {
		t.Fatalf("saveDB: %v", err)
	}
	if _, err := os.Stat(dbPath(root) + ".tmp"); err == nil {
		t.Error("temporary db file was left behind")
	}
	got, err := loadDB(root)
	if err != nil {
		t.Fatalf("loadDB: %v", err)
	}
	if got.Packages["hello"].Version != "1.0.0" {
		t.Errorf("round-trip lost data: %+v", got.Packages)
	}
}

func TestInstallRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(t.TempDir(), "evil.pkg")
	writePkg(t, pkg, []pkgEntry{{name: "files/../../pwned.txt", body: "boom"}})

	if code := cmdInstall([]string{"-root", root, pkg}); code == 0 {
		t.Fatal("cmdInstall accepted a package with a traversing path")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "pwned.txt")); err == nil {
		t.Fatal("traversal wrote a file outside the target root")
	}
}

func TestInstallValidPackage(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(t.TempDir(), "hello.pkg")
	writePkg(t, pkg, []pkgEntry{{name: "files/usr/bin/hello", body: "#!/bin/sh\n"}})

	if code := cmdInstall([]string{"-root", root, pkg}); code != 0 {
		t.Fatalf("cmdInstall returned %d for a valid package", code)
	}
	got := filepath.Join(root, "usr", "bin", "hello")
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("installed file missing: %v", err)
	}
	db, err := loadDB(root)
	if err != nil {
		t.Fatalf("loadDB: %v", err)
	}
	if _, ok := db.Packages["evil"]; !ok {
		t.Fatalf("package not recorded in db: %+v", db.Packages)
	}
}
