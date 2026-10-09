//go:build linux

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type mountSpec struct {
	source string
	target string
	fstype string
	flags  uintptr
	data   string
}

var mountSpecs = []mountSpec{
	{"proc", "/proc", "proc", syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC, ""},
	{"sysfs", "/sys", "sysfs", syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC, ""},
	{"devtmpfs", "/dev", "devtmpfs", 0, "mode=0755"},
	{"tmpfs", "/run", "tmpfs", syscall.MS_NOSUID | syscall.MS_NODEV, "mode=0755"},
	{"tmpfs", "/tmp", "tmpfs", syscall.MS_NOSUID | syscall.MS_NODEV, "mode=1777"},
}

func main() {
	fmt.Println("mininit: starting as PID", os.Getpid())

	for _, m := range mountSpecs {
		if err := syscall.Mkdir(m.target, 0o755); err != nil && !os.IsExist(err) {
			fmt.Printf("mininit: mkdir %s: %v\n", m.target, err)
			continue
		}
		if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, m.data); err != nil {
			if err == syscall.EBUSY {
				fmt.Printf("mininit: %s already mounted\n", m.target)
				continue
			}
			fmt.Printf("mininit: mount %s: %v\n", m.target, err)
			continue
		}
		fmt.Printf("mininit: mounted %s (%s)\n", m.target, m.fstype)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		fmt.Println("mininit: powering off")
		syscall.Sync()
		syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
	}()

	for {
		pid, err := startShell()
		if err != nil {
			fmt.Printf("mininit: no usable shell: %v (retrying in 1s)\n", err)
			time.Sleep(time.Second)
			continue
		}
		fmt.Printf("mininit: started shell (pid %d)\n", pid)
		waitForShell(pid)
	}
}

type shellCandidate struct {
	path string
	argv []string
}

var shellCandidates = []shellCandidate{
	{"/bin/sh", []string{"/bin/sh", "-l"}},
	{"/bin/busybox", []string{"/bin/busybox", "sh", "-l"}},
}

func startShell() (int, error) {
	env := []string{"PATH=/usr/bin:/bin:/sbin:/usr/sbin", "HOME=/root", "TERM=linux", "PS1=minidistro# "}
	var lastErr error
	for _, c := range shellCandidates {
		pid, err := syscall.ForkExec(c.path, c.argv, &syscall.ProcAttr{
			Dir:   "/root",
			Files: []uintptr{0, 1, 2},
			Env:   env,
		})
		if err == nil {
			return pid, nil
		}
		lastErr = err
	}
	return 0, lastErr
}

func waitForShell(shellPid int) {
	for {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, 0, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			fmt.Printf("mininit: wait: %v\n", err)
			return
		}
		if pid == shellPid {
			fmt.Printf("mininit: shell exited (status %d) — respawning\n", ws.ExitStatus())
			return
		}
		fmt.Printf("mininit: reaped orphan pid %d (status %d)\n", pid, ws.ExitStatus())
	}
}
