package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "0.2.0"

const (
	kernelPath = "workspace/sources/bzImage"
	initrdPath = "workspace/build/initramfs.cpio.gz"
	rootfsPath = "workspace/rootfs"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "check":
		os.Exit(runCheck())
	case "init":
		os.Exit(runInit())
	case "build":
		os.Exit(runBuild())
	case "run":
		os.Exit(runVM())
	case "version", "--version":
		fmt.Println("distroforge", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "distroforge: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`distroforge — companion CLI for the build-your-own-distro course

Usage:
  distroforge check     verify toolchain, QEMU, and workspace
  distroforge init      scaffold the workspace (sources/, build/, rootfs/, iso/)
  distroforge build     compile mininit and pack the initramfs
  distroforge run       boot the image in QEMU
  distroforge version   print version
`)
}

func runCheck() int {
	fmt.Println("build-your-own-distro — environment check")
	fmt.Println(strings.Repeat("-", 44))

	if runtime.GOOS == "linux" {
		fmt.Println("  [ ok ] OS: Linux")
	} else {
		fmt.Println("  [warn] OS:", runtime.GOOS+"/"+runtime.GOARCH,
			"— build and run work here; chapter commands assume a Linux shell")
	}

	type tool struct {
		name string
		req  bool
		hint string
	}
	tools := []tool{
		{"go", true, "install from https://go.dev/dl/"},
		{"qemu-system-x86_64", false, installHint("qemu-system-x86")},
		{"gcc", false, installHint("build-essential")},
		{"make", false, installHint("build-essential")},
		{"xorriso", false, installHint("xorriso")},
		{"grub-mkrescue", false, installHint("grub-pc-bin")},
	}

	missingRequired := 0
	missingOptional := 0
	for _, t := range tools {
		ok := has(t.name)
		status := "ok  "
		if !ok {
			if t.req {
				status = "FAIL"
				missingRequired++
			} else {
				status = " .. "
				missingOptional++
			}
		}
		fmt.Printf("  [%s] %s\n", status, t.name)
		if !ok {
			fmt.Printf("         → %s\n", t.hint)
		}
	}

	if _, err := os.Stat("workspace"); err == nil {
		fmt.Println("  [ ok ] workspace/ exists")
	} else {
		fmt.Println("  [ .. ] workspace/ missing — run `distroforge init`")
	}

	fmt.Println(strings.Repeat("-", 44))
	if missingRequired > 0 {
		fmt.Printf("%d required tool(s) missing — fix them first\n", missingRequired)
		return 1
	}
	if missingOptional > 0 {
		fmt.Printf("ready (%d optional tool(s) missing — needed by later chapters))\n", missingOptional)
		return 0
	}
	fmt.Println("ready — start with chapters/00-what-is-a-distro.md")
	return 0
}

func installHint(pkg string) string {
	if runtime.GOOS == "windows" {
		switch {
		case strings.Contains(pkg, "qemu"):
			return "winget install SoftwareFreedomConservancy.QEMU"
		case strings.Contains(pkg, "gcc") || strings.Contains(pkg, "make"):
			return "winget install MSYS2.MSYS2, then: pacman -S make gcc"
		default:
			return "Windows: install MSYS2 or use WSL2 (see chapters/01-environment.md)"
		}
	}
	return "apt install " + pkg + " / pacman -S " + pkg
}

func has(tool string) bool {
	if tool == "qemu-system-x86_64" {
		return qemuPath() != ""
	}
	_, err := exec.LookPath(tool)
	return err == nil
}

func qemuPath() string {
	if p, err := exec.LookPath("qemu-system-x86_64"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			`C:\Program Files\qemu\qemu-system-x86_64.exe`,
			`C:\Program Files (x86)\qemu\qemu-system-x86_64.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func runInit() int {
	dirs := []string{
		"workspace/sources",
		"workspace/build",
		"workspace/iso",
		rootfsPath + "/bin",
		rootfsPath + "/boot",
		rootfsPath + "/dev",
		rootfsPath + "/etc",
		rootfsPath + "/home",
		rootfsPath + "/lib",
		rootfsPath + "/media",
		rootfsPath + "/mnt",
		rootfsPath + "/opt",
		rootfsPath + "/proc",
		rootfsPath + "/root",
		rootfsPath + "/run",
		rootfsPath + "/sbin",
		rootfsPath + "/srv",
		rootfsPath + "/sys",
		rootfsPath + "/tmp",
		rootfsPath + "/usr/bin",
		rootfsPath + "/usr/lib",
		rootfsPath + "/usr/share",
		rootfsPath + "/var/cache",
		rootfsPath + "/var/log",
		rootfsPath + "/var/lib",
		rootfsPath + "/var/lib/pkg",
	}

	created := 0
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "init: %v\n", err)
			return 1
		}
		created++
	}
	for _, d := range []string{rootfsPath + "/tmp", rootfsPath + "/run"} {
		if err := os.Chmod(d, 0o1777); err != nil {
			fmt.Fprintf(os.Stderr, "init: chmod %s: %v\n", d, err)
			return 1
		}
	}

	files := map[string]string{
		rootfsPath + "/etc/passwd": "root:x:0:0:root:/root:/bin/sh\n",
		rootfsPath + "/etc/group":  "root:x:0:\n",
		rootfsPath + "/etc/fstab":  "# <fs>      <mountpoint>  <type>  <options>  <dump> <pass>\nproc        /proc         proc    defaults   0      0\nsysfs       /sys          sysfs   defaults   0      0\ntmpfs       /run          tmpfs   mode=0755  0      0\n",
		rootfsPath + "/etc/os-release": "NAME=\"Minidistro\"\nID=minidistro\nVERSION=\"0.1\"\nPRETTY_NAME=\"Minidistro 0.1 (from-scratch)\"\nHOME_URL=\"https://github.com/arcnosixta/build-your-own-distro\"\n",
		rootfsPath + "/etc/issue":  "Minidistro \\r \\l\n\n",
		rootfsPath + "/etc/motd":   "Welcome to Minidistro — built from scratch.\nEdit /etc/profile to customize your shell.\n",
		rootfsPath + "/etc/profile": "# /etc/profile — system-wide shell startup\n" +
			"[ -r /etc/os-release ] && . /etc/os-release\n" +
			"export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n" +
			"export PS1='\\u@minidistro:\\w# '\n" +
			"if [ -r /etc/motd ]; then while IFS= read -r line; do echo \"$line\"; done < /etc/motd; fi\n",
		rootfsPath + "/root/.profile": "# ~/.profile — per-user startup (a login shell reads /etc/profile first)\n# Add personal aliases and environment tweaks here.\n",
	}
	createdFiles := 0
	for path, content := range files {
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "init: %v\n", err)
			return 1
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "init: %v\n", err)
			return 1
		}
		createdFiles++
	}

	if created > 0 || createdFiles > 0 {
		fmt.Printf("created %d directories and %d base files in workspace/\n", created, createdFiles)
	} else {
		fmt.Println("workspace/ already up to date")
	}
	fmt.Println("next: read chapters/02-rootfs.md, then chapters/03-kernel.md")
	return 0
}

func runBuild() int {
	if _, err := os.Stat(rootfsPath); err != nil {
		fmt.Fprintln(os.Stderr, "build: workspace/ missing — run `distroforge init` first")
		return 1
	}

	fmt.Println("stage 1/4: compile mininit (static linux/amd64)")
	if !buildBinary("./cmd/mininit", filepath.Join("workspace", "build", "mininit")) {
		return 1
	}

	fmt.Println("stage 2/4: compile pkg (static linux/amd64)")
	if !buildBinary("./cmd/pkg", filepath.Join("workspace", "build", "pkg")) {
		return 1
	}

	fmt.Println("stage 3/4: install binaries into rootfs")
	if err := copyFile(filepath.Join("workspace", "build", "mininit"), filepath.Join(rootfsPath, "sbin", "init"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n", err)
		return 1
	}
	if err := copyFile(filepath.Join("workspace", "build", "pkg"), filepath.Join(rootfsPath, "sbin", "pkg"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n", err)
		return 1
	}
	fmt.Println("        installed rootfs/sbin/init and rootfs/sbin/pkg")

	fmt.Println("stage 4/4: pack initramfs")
	files, size, err := packInitramfs(rootfsPath, initrdPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build: pack: %v\n", err)
		return 1
	}
	fmt.Printf("        %s: %d entries, %.1f MiB\n", initrdPath, files, float64(size)/(1024*1024))

	if _, err := os.Stat(kernelPath); err != nil {
		fmt.Printf("\nimage ready, but no kernel yet — finish chapters/03-kernel.md (%s)\n", kernelPath)
		return 0
	}
	fmt.Println("\nbootable — start it with: distroforge run")
	return 0
}

func buildBinary(pkg, out string) bool {
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %s: %v\n", pkg, err)
		return false
	}
	fmt.Println("        compiled", out)
	return true
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func runVM() int {
	if _, err := os.Stat(kernelPath); err != nil {
		fmt.Fprintf(os.Stderr, "run: %s missing — finish chapters/03-kernel.md first\n", kernelPath)
		return 1
	}
	if _, err := os.Stat(initrdPath); err != nil {
		fmt.Fprintln(os.Stderr, "run: initramfs missing — run `distroforge build` first")
		return 1
	}
	qemu := qemuPath()
	if qemu == "" {
		fmt.Fprintf(os.Stderr, "run: qemu-system-x86_64 not found — %s\n", installHint("qemu-system-x86"))
		return 1
	}

	args := []string{"-m", "512", "-nographic", "-no-reboot"}
	if _, err := os.Stat("/dev/kvm"); err == nil {
		args = append(args, "-enable-kvm", "-cpu", "host")
	}
	args = append(args,
		"-kernel", kernelPath,
		"-initrd", initrdPath,
		"-append", "console=ttyS0 rdinit=/sbin/init",
	)

	fmt.Printf("+ %s %s\n", qemu, strings.Join(args, " "))
	fmt.Println("  (quit QEMU with Ctrl-A then X)")
	cmd := exec.Command(qemu, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "run: %v\n", err)
		return 1
	}
	return 0
}
