# Chapter 03 — Kernel: build it or borrow it

Everything we have built so far is inert: a directory tree with no way to run.
The kernel is the program that turns that tree into a system. It is also the
one component almost nobody writes themselves — so this chapter teaches you
where the kernel comes from, how to get one four different ways, and why a
`bzImage` is the only file the bootloader actually needs.

## What the kernel is responsible for

Once it boots, the kernel is the only code running in ring 0. It owns:

- **Memory** — virtual address spaces, paging, the page cache
- **Processes** — scheduling, signals, the process table (`/proc`)
- **Devices** — drivers, block/character devices (`/dev`)
- **Filesystems** — VFS, and every actual filesystem (ext4, tmpfs, procfs…)
- **The syscall interface** — the only door from userland into all of the above

Your init, your shell, and your package manager are all just programs making
syscalls. The kernel is the syscall table.

## `vmlinuz` vs `bzImage`

Two names for one artifact:

| Name | Where you see it | What it is |
|------|------------------|------------|
| `bzImage` | build output: `arch/x86/boot/bzImage` | the kernel image format |
| `vmlinuz` | installed distros: `/boot/vmlinuz-*` | a `bzImage` with a size hack ("z" = compressed, "z" = it's the classic name) |

For QEMU's `-kernel` flag we just need the bytes — either name works. We save
ours as `workspace/sources/bzImage` to match the kernel's own terminology.

## Get a kernel — four ways

Pick one. They all end with a kernel at `workspace/sources/bzImage`.

### Option A — Borrow the running one (fastest, Linux only)

If you are on a Linux shell, the kernel you are using right now is already a
perfectly good kernel:

```bash
cp /boot/vmlinuz-$(uname -r) workspace/sources/bzImage
```

This is the honest shortcut: a distribution is *not* a kernel you compiled, it
is a kernel plus everything around it.

### Option B — Extract a distro package (Debian/Ubuntu/WSL2)

```bash
cd workspace/sources
apt download linux-image-generic
dpkg-deb -x linux-image-*.deb extracted/
cp extracted/boot/vmlinuz-*-generic bzImage
```

You just learned where Ubuntu's `/boot/vmlinuz` comes from: it is a file inside
a `.deb` that `dpkg` normally unpacks for you.

### Option C — Download a prebuilt kernel (works on Windows, no compiler)

Any prebuilt distribution kernel is fine. The Alpine "virt" kernel is small,
has built-in serial and virtio drivers, and boots cleanly under QEMU:

```bash
curl -fL -o workspace/sources/bzImage \
  https://dl-cdn.alpinelinux.org/alpine/latest-stable/releases/x86_64/netboot/vmlinuz-virt
```

This is the path used by the project's own verification run on Windows, and it
is the one to pick if you are following along without a Linux toolchain.

### Option D — Build it from source (the real thing, ~20–40 min)

Only if you want the full LFS experience. The config matters more than the
build:

```bash
cd workspace/sources
curl -fLO https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-6.12.tar.xz
tar -xf linux-6.12.tar.xz
cd linux-6.12
make defconfig                      # a sane, generic starting point
scripts/config --enable CONFIG_DEVTMPFS
scripts/config --enable CONFIG_DEVTMPFS_MOUNT
scripts/config --enable CONFIG_SERIAL_8250
scripts/config --enable CONFIG_SERIAL_8250_CONSOLE
make -j"$(nproc)"
cp arch/x86/boot/bzImage ../../sources/bzImage
```

The four `scripts/config` lines are the whole lesson of *configuring* a kernel:
`defconfig` is generic, and a bootable image is a defconfig plus the handful of
options your specific hardware (here: a virtual machine with a serial console)
requires. Build dependencies: `bc bison flex libssl-dev`.

## A working kernel is not a working system

Give QEMU your borrowed kernel and nothing else:

```bash
qemu-system-x86_64 -m 512 -kernel workspace/sources/bzImage -nographic
```

The kernel prints a hundred lines of hardware detection and then panics:

```
Kernel panic - not syncing: No working init found.
```

Read that panic carefully — it is the kernel telling you exactly what is
missing. It looked for `/sbin/init`, `/etc/init`, `/bin/init`, `/bin/sh`, found
none it could execute, and gave up. Chapter 04 provides the missing piece.

## Check yourself

1. Why can the same kernel boot Ubuntu, Fedora, and Minidistro?
2. What does `CONFIG_DEVTMPFS_MOUNT` do, and why do we want it?
3. Why does the kernel panic *after* printing hardware messages instead of
   before?

## Next

[Chapter 04 — Init: your first process (PID 1) in Go](04-init.md): give the
kernel a `/sbin/init`, and boot for the first time.
