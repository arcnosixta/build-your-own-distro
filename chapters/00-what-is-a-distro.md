# Chapter 00 — What is a Linux distribution?

A kernel is not an operating system. Kernel.org ships a compressed tarball that
can boot on nothing; everything you recognize as "Ubuntu" or "Fedora" is what
people build *around* that kernel. This chapter maps those pieces, because the
rest of the course is nothing but assembling them one by one.

## The eight layers of a distribution

| Layer | What it does | Who provides it |
|-------|--------------|-----------------|
| Bootloader | Finds the kernel and hands over control | GRUB, systemd-boot |
| Kernel | Manages hardware, memory, processes | kernel.org, or your build |
| Init (PID 1) | The first process; starts everything else | systemd, OpenRC — or yours |
| Userland | Shells, coreutils, libraries | busybox, GNU, musl |
| Package manager | Installs, upgrades, removes software with a database | apt, pacman, dnf — or 500 lines of Go |
| Repositories | Signed collections of packages | your hosting |
| Installer | Puts it all on a disk | calamares, or a shell script |
| Configuration | Defaults, services, policies | you |

A distribution is the *choice* of these components plus the glue that makes
them boot together. That is why there are a thousand distros and only one
kernel: changing a default wallpaper is a distro; changing the syscall table
is not.

## What we are building

"Minidistro" — small enough to understand completely:

- **Boots in QEMU** from a single ISO you built yourself
- **Own init** — a PID 1 written in Go (Chapter 04), so you feel what init does
- **Own package manager** — tar+zstd archives with a JSON database (Chapter 06)
- **Reproducible workspace** — every step is a command you can re-run

What we deliberately skip: an installer UI, a desktop environment, and
rebuilding glibc from source. LFS already covers from-source compilation; this
course covers *architecture* — how the pieces connect and boot.

## Why Go for the system parts

Init and the package manager are small daemons that shell out to the kernel
and manage files. Go gives us static binaries (no libc dependency in the
rootfs), fast startup, and a memory-safe implementation — unlike the classic
C-from-scratch tutorials, a bug in our init won't silently corrupt memory.

## The mental model to keep

```
power on → bootloader → kernel → kernel starts /sbin/init (PID 1)
        → init mounts pseudo-filesystems → starts services
        → you see a login prompt
```

Every chapter from here on fills in exactly one box of that arrow diagram.

## Check yourself

1. Name three components that differ between Ubuntu and Fedora but not
   between them and Arch.
2. Why can a distribution ship a different init but not a different kernel
   (without becoming a fork)?
3. What is PID 1, and why must it never exit?

## Next

[Chapter 01 — Your build environment](01-environment.md): set up the
workspace where our distro will be assembled.
