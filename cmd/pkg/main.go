package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const version = "0.1.0"

type manifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Depends     []string `json:"depends,omitempty"`
}

type entry struct {
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Files       []string `json:"files"`
}

type database struct {
	Packages map[string]entry `json:"packages"`
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "build":
		os.Exit(cmdBuild(args[1:]))
	case "install":
		os.Exit(cmdInstall(args[1:]))
	case "remove":
		os.Exit(cmdRemove(args[1:]))
	case "list":
		os.Exit(cmdList(args[1:]))
	case "info":
		os.Exit(cmdInfo(args[1:]))
	case "version", "--version":
		fmt.Println("pkg", version, "(minidistro)")
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "pkg: unknown command %q\n", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`pkg — the Minidistro package manager

Usage:
  pkg build  -name NAME -root DIR [-version V] [-desc TEXT] [-out FILE]
  pkg install [-root DIR] FILE.pkg
  pkg remove  [-root DIR] NAME
  pkg list    [-root DIR]
  pkg info    [-root DIR] NAME

-root defaults to / (the running system). On a build host, point it at
workspace/rootfs to install packages into the image offline.
`)
}

func dbPath(root string) string {
	return filepath.Join(root, "var", "lib", "pkg", "db.json")
}

func loadDB(root string) (*database, error) {
	db := &database{Packages: map[string]entry{}}
	b, err := os.ReadFile(dbPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return db, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, db); err != nil {
		return nil, fmt.Errorf("database is corrupt: %w", err)
	}
	if db.Packages == nil {
		db.Packages = map[string]entry{}
	}
	return db, nil
}

func saveDB(root string, db *database) error {
	if err := os.MkdirAll(filepath.Dir(dbPath(root)), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dbPath(root), append(b, '\n'), 0o644)
}

func cmdBuild(argv []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	name := fs.String("name", "", "package name")
	ver := fs.String("version", "0.0.0", "package version")
	desc := fs.String("desc", "", "short description")
	src := fs.String("root", "", "directory tree to package")
	out := fs.String("out", "", "output file (default NAME-VERSION.pkg)")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	if *name == "" || *src == "" {
		fmt.Fprintln(os.Stderr, "pkg build: -name and -root are required")
		return 2
	}
	if *out == "" {
		*out = *name + "-" + *ver + ".pkg"
	}

	man := manifest{Name: *name, Version: *ver, Description: *desc}
	mb, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg build:", err)
		return 1
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg build:", err)
		return 1
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	count := 1
	err = writeTarFile(tw, "manifest.json", mb, 0o644, time.Now())
	if err == nil {
		err = filepath.Walk(*src, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(*src, p)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			name := "files/" + filepath.ToSlash(rel)
			count++
			switch {
			case info.IsDir():
				return writeTarDir(tw, name, 0o755, info.ModTime())
			case info.Mode()&os.ModeSymlink != 0:
				target, err := os.Readlink(p)
				if err != nil {
					return err
				}
				return writeTarLink(tw, name, target, info.ModTime())
			default:
				in, err := os.Open(p)
				if err != nil {
					return err
				}
				defer in.Close()
				return streamTarFile(tw, name, fileMode(name, info), info.ModTime(), in)
			}
		})
	}

	if cerr := tw.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if cerr := gz.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if cerr := f.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(*out)
		fmt.Fprintln(os.Stderr, "pkg build:", err)
		return 1
	}

	st, _ := os.Stat(*out)
	fmt.Printf("built %s (%s %s, %d entries, %.1f KiB)\n",
		*out, man.Name, man.Version, count, float64(st.Size())/1024)
	return 0
}

func cmdInstall(argv []string) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	root := fs.String("root", "/", "target root filesystem")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "pkg install: exactly one package file is required")
		return 2
	}

	f, err := os.Open(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg install:", err)
		return 1
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg install:", err)
		return 1
	}
	tr := tar.NewReader(gz)

	var man manifest
	var files []string
	seen := map[string]bool{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "pkg install:", err)
			return 1
		}
		if hdr.Name == "manifest.json" {
			if err := json.NewDecoder(tr).Decode(&man); err != nil {
				fmt.Fprintln(os.Stderr, "pkg install: bad manifest:", err)
				return 1
			}
			continue
		}
		if !strings.HasPrefix(hdr.Name, "files/") {
			continue
		}
		rel := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "files/")
		target := filepath.Join(*root, filepath.FromSlash(rel))

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)&0o777); err != nil {
				fmt.Fprintln(os.Stderr, "pkg install:", err)
				return 1
			}
			continue
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				fmt.Fprintln(os.Stderr, "pkg install:", err)
				return 1
			}
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				fmt.Fprintf(os.Stderr, "pkg install: symlink %s skipped: %v\n", rel, err)
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				fmt.Fprintln(os.Stderr, "pkg install:", err)
				return 1
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				fmt.Fprintln(os.Stderr, "pkg install:", err)
				return 1
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				fmt.Fprintln(os.Stderr, "pkg install:", err)
				return 1
			}
			out.Close()
		}
		abs := "/" + rel
		if !seen[abs] {
			seen[abs] = true
			files = append(files, abs)
		}
	}

	if man.Name == "" {
		fmt.Fprintln(os.Stderr, "pkg install: package has no manifest.json")
		return 1
	}

	db, err := loadDB(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg install:", err)
		return 1
	}
	if old, ok := db.Packages[man.Name]; ok {
		for _, f := range old.Files {
			if !seen[f] {
				os.Remove(filepath.Join(*root, filepath.FromSlash(f)))
			}
		}
	}
	sort.Strings(files)
	db.Packages[man.Name] = entry{Version: man.Version, Description: man.Description, Files: files}
	if err := saveDB(*root, db); err != nil {
		fmt.Fprintln(os.Stderr, "pkg install:", err)
		return 1
	}

	fmt.Printf("installed %s %s (%d files)\n", man.Name, man.Version, len(files))
	return 0
}

func cmdRemove(argv []string) int {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	root := fs.String("root", "/", "target root filesystem")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "pkg remove: exactly one package name is required")
		return 2
	}
	name := rest[0]

	db, err := loadDB(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg remove:", err)
		return 1
	}
	e, ok := db.Packages[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "pkg remove: %s is not installed\n", name)
		return 1
	}
	for _, f := range e.Files {
		os.Remove(filepath.Join(*root, filepath.FromSlash(f)))
	}
	delete(db.Packages, name)
	if err := saveDB(*root, db); err != nil {
		fmt.Fprintln(os.Stderr, "pkg remove:", err)
		return 1
	}
	fmt.Printf("removed %s %s (%d files)\n", name, e.Version, len(e.Files))
	return 0
}

func cmdList(argv []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	root := fs.String("root", "/", "target root filesystem")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	db, err := loadDB(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg list:", err)
		return 1
	}
	if len(db.Packages) == 0 {
		fmt.Println("no packages installed")
		return 0
	}
	names := make([]string, 0, len(db.Packages))
	for n := range db.Packages {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		e := db.Packages[n]
		if e.Description != "" {
			fmt.Printf("%-20s %-10s %s\n", n, e.Version, e.Description)
		} else {
			fmt.Printf("%-20s %s\n", n, e.Version)
		}
	}
	return 0
}

func cmdInfo(argv []string) int {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	root := fs.String("root", "/", "target root filesystem")
	if err := fs.Parse(argv); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "pkg info: exactly one package name is required")
		return 2
	}
	db, err := loadDB(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pkg info:", err)
		return 1
	}
	e, ok := db.Packages[rest[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "pkg info: %s is not installed\n", rest[0])
		return 1
	}
	fmt.Printf("name:    %s\nversion: %s\n", rest[0], e.Version)
	if e.Description != "" {
		fmt.Printf("desc:    %s\n", e.Description)
	}
	fmt.Printf("files:   %d\n", len(e.Files))
	for _, f := range e.Files {
		fmt.Printf("  %s\n", f)
	}
	return 0
}

func isExecPath(name string) bool {
	for _, d := range []string{"files/bin/", "files/sbin/", "files/usr/bin/", "files/usr/sbin/", "files/usr/local/bin/", "files/usr/local/sbin/"} {
		if strings.HasPrefix(name, d) {
			return true
		}
	}
	return false
}

func fileMode(name string, info os.FileInfo) os.FileMode {
	perm := info.Mode().Perm()
	if perm&0o111 != 0 || isExecPath(name) {
		return 0o755
	}
	return 0o644
}

func writeTarFile(tw *tar.Writer, name string, data []byte, mode os.FileMode, mtime time.Time) error {
	hdr := &tar.Header{
		Name:     name,
		Mode:     int64(mode) & 0o777,
		Size:     int64(len(data)),
		ModTime:  mtime,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func streamTarFile(tw *tar.Writer, name string, mode os.FileMode, mtime time.Time, r io.Reader) error {
	st, ok := r.(interface{ Stat() (os.FileInfo, error) })
	var size int64
	if ok {
		if fi, err := st.Stat(); err == nil {
			size = fi.Size()
		}
	}
	hdr := &tar.Header{
		Name:     name,
		Mode:     int64(mode) & 0o777,
		Size:     size,
		ModTime:  mtime,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := io.CopyN(tw, r, size)
	return err
}

func writeTarDir(tw *tar.Writer, name string, mode os.FileMode, mtime time.Time) error {
	return tw.WriteHeader(&tar.Header{
		Name:     name + "/",
		Mode:     int64(mode) & 0o777,
		ModTime:  mtime,
		Typeflag: tar.TypeDir,
	})
}

func writeTarLink(tw *tar.Writer, name, target string, mtime time.Time) error {
	return tw.WriteHeader(&tar.Header{
		Name:     name,
		Linkname: target,
		ModTime:  mtime,
		Typeflag: tar.TypeSymlink,
	})
}
