# Chapter 04 — Init: your first process (PID 1) in Go

The kernel just told us it has nothing to run. This chapter hands it a program
we wrote ourselves — a PID 1 in Go — and boots a real, if tiny, Linux system in
QEMU. This is the milestone the whole course has been walking toward.

## What PID 1 actually is

When the kernel finishes booting it starts exactly one userspace program: the
one named on the command line (`rdinit=` or `init=`) or a list of defaults, and
it gives that program **PID 1**. PID 1 is special:

- It cannot die. If PID 1 exits, the kernel panics.
- It inherits every orphaned process. When any process's parent dies, the
  orphan is reparented to PID 1, and PID 1 must `wait()` on it — otherwise the
  process table fills with zombies.
- It receives the poweroff/reboot signals (`SIGTERM`, from `reboot(8)`), because
  only PID 1 can ask the kernel to shut down cleanly.

That is the entire job description. systemd is PID 1 plus thousands of features;
our `mininit` is PID 1 and nothing else, which is exactly why it is worth
writing once.

## The init we build

The source lives in `cmd/mininit/main.go`. It does four things, in order:

1. **Mount the pseudo-filesystems** the kernel exposes but does not mount for
   you: `/proc`, `/sys`, `/dev`, `/run`, `/tmp`. Until these exist, almost
   nothing works — this is why early boot always breaks without them.
2. **Install a signal handler** so `SIGTERM`/`SIGINT` syncs the disks and powers
   off cleanly instead of panicking the kernel.
3. **Start a shell** as a child process.
4. **Loop as the reaper**: block in `wait4()` forever, reap orphans, and
   respawn the shell if it exits.

The interesting parts are the syscalls. The mounts:

```go
syscall.Mount("proc", "/proc", "proc",
    syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "")
```

and the shell, started with `ForkExec` and the same three standard file
descriptors inherited from PID 1 (serial console):

```go
syscall.ForkExec("/bin/sh", []string{"/bin/sh", "-i"}, &syscall.ProcAttr{
    Dir:   "/root",
    Files: []uintptr{0, 1, 2},
    Env:   env,
})
```

A nice consequence of writing init in Go: it compiles to a **static binary**.
No libc, no dynamic loader, no `/lib` — one file that runs the instant the
kernel hands over control. Most init-crashes in the real world are a missing
shared library; ours cannot have that bug.

## Get a shell: busybox

Our init needs something to execute. `busybox` is one static binary that
provides `sh`, `ls`, `cat`, `mount`, `ps`, and hundreds more, selected by the
name it is called as. Get it the same way you got the kernel:

```bash
# Debian/Ubuntu/WSL2
cd workspace/sources
apt download busybox-static
dpkg-deb -x busybox-static_*.deb extracted/
cp extracted/bin/busybox ../rootfs/bin/busybox
```

```bash
# any platform, prebuilt static binary
curl -fL -o workspace/rootfs/bin/busybox \
  https://busybox.net/downloads/binaries/1.35.0-x86_64-linux-musl/busybox
```

On a Linux shell, also create the usual command symlinks so `/bin/sh` resolves:

```bash
cd workspace/rootfs/bin
for c in sh ls cat mount echo sleep ps mkdir; do ln -sf busybox "$c"; done
```

If you skip the symlinks (as you must on Windows, which cannot make them), the
init falls back to running `/bin/busybox sh` directly — the same shell, reached
by a different path.

## Build the image

```bash
distroforge build
```

Three stages:

```
stage 1/3: compile mininit (static linux/amd64)   # GOOS=linux, CGO_ENABLED=0
stage 2/3: install /sbin/init into rootfs         # copy mininit → rootfs/sbin/init
stage 3/3: pack initramfs                          # rootfs → initramfs.cpio.gz
```

Stage 3 deserves a note. A boot initramfs is a **cpio `newc` archive, gzip
compressed** — the format the kernel unpacks into a RAM disk. Rather than shell
out to `find | cpio | gzip`, `distroforge` builds it in pure Go (see
`cli/initramfs.go`), which means no `cpio` dependency and identical output on
every OS. It also fixes a classic trap: file permissions. A `/sbin/init` without
the execute bit makes the kernel print `Failed to execute /sbin/init (error -13)`
and panic — permission bits are part of the boot contract.

## Boot it

```bash
distroforge run
```

which is a thin wrapper around:

```bash
qemu-system-x86_64 -m 512 -nographic -no-reboot \
  -kernel workspace/sources/bzImage \
  -initrd workspace/build/initramfs.cpio.gz \
  -append "console=ttyS0 rdinit=/sbin/init"
```

- `-kernel` / `-initrd`: give QEMU the kernel and the RAM disk directly — no
  bootloader yet (that is Chapter 07).
- `-append "rdinit=/sbin/init"`: tells the kernel to run our init out of the
  initramfs.
- `-nographic`: send the serial console to your terminal instead of opening a
  GUI window. **Quit QEMU with `Ctrl-A` then `X`.**

The kernel logs scroll by, and then — after the line `Run /sbin/init as init
process` — your code takes over:

```
mininit: starting as PID 1
mininit: mounted /proc (proc)
mininit: mounted /sys (sysfs)
mininit: mounted /dev (devtmpfs)
mininit: mounted /run (tmpfs)
mininit: mounted /tmp (tmpfs)
mininit: started shell (pid 434)
sh: can't access tty; job control turned off
minidistro#
```

That prompt is a Linux system you assembled: your kernel, your rootfs, your
init, your shell. Everything from Chapter 05 onward makes it useful.

## Why this is an initramfs, not a disk

Notice there is no hard drive in the `run` command. The root filesystem lives
entirely in RAM, unpacked from the cpio archive at boot. Real distros do the
same thing for their first few milliseconds — load a tiny initramfs to mount the
*real* root, then `switch_root`. Chapters 07 and 08 build an actual disk image
and move to it; here we stop early precisely so you can see how little a
"bootable system" requires.

## Check yourself

1. The kernel reaps orphaned processes through PID 1. What happens if PID 1
   does not call `wait()`?
2. Why does a statically linked init survive scenarios where a dynamic one
   would fail to start?
3. Our init mounts `/dev` even though modern kernels can auto-mount
   `devtmpfs`. When is it still the init's job?

## Next

[Chapter 05 — Userland: from busybox to coreutils](05-userland.md): turn a shell
into an environment — libraries, login, and the little programs a system
expects.
