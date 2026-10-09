# Build Your Own Linux Distro

> A hands-on course + Go CLI that takes you from an empty folder to a bootable Linux distribution — and explains every layer you just built.

Linux From Scratch taught a generation how a distribution is assembled, but it is a PDF book frozen in a print-era format. This project is the same journey rebuilt for GitHub: a chapter-by-chapter course with a companion CLI tool that scaffolds, verifies, and builds each step, so you spend your time on understanding instead of on debugging a typo in a 600-page manual.

## What you get

- **A course** — 12 chapters, from "what is a distro?" to a bootable ISO with its own package manager, running in QEMU.
- **A CLI (`distroforge`)** — a single Go binary that checks your environment, scaffolds each chapter's workspace, and drives the build, so every command in the text is copy-pasteable and verified.

## Who this is for

Developers who want a mental model of the system they work on every day: where `/sbin/init` comes from, what a bootloader actually loads, why package managers need a database, and what "immutable OS" means because you just built a mutable one.

## Course curriculum

| # | Chapter | Status |
|---|---------|--------|
| 00 | [What is a Linux distribution?](chapters/00-what-is-a-distro.md) | ✅ |
| 01 | [Your build environment (WSL2, QEMU, disk)](chapters/01-environment.md) | ✅ |
| 02 | [The root filesystem](chapters/02-rootfs.md) | ✅ |
| 03 | [Kernel: build it or borrow it](chapters/03-kernel.md) | ✅ |
| 04 | [Init: your first process (PID 1) in Go](chapters/04-init.md) | ✅ |
| 05 | [Userland: busybox to coreutils](chapters/05-userland.md) | ✅ |
| 06 | [A package manager in 500 lines](chapters/06-packages.md) | 📝 |
| 07 | [Bootloader and UEFI](chapters/07-bootloader.md) | 📝 |
| 08 | [From rootfs to bootable ISO](chapters/08-iso.md) | 📝 |
| 09 | [Updates: A/B slots and immutability](chapters/09-updates.md) | 📝 |
| 10 | [Hardening your distro](chapters/10-hardening.md) | 📝 |
| 11 | [How the industry does it: mkosi, bootc, Universal Blue](chapters/11-industry.md) | 📝 |

✅ published · 📝 drafted next

## Quick start

Prerequisites: QEMU (a Linux shell is recommended; Windows-native build/run is
supported via a prebuilt kernel). Only kernel-from-source needs a compiler and
~40 GB of disk.

```bash
# 1. Clone the course
git clone https://github.com/arcnosixta/build-your-own-distro.git
cd build-your-own-distro

# 2. Check your environment
go run ./cli check

# 3. Start Chapter 00
cat chapters/00-what-is-a-distro.md
```

## The CLI

```
distroforge check    # verify toolchain, QEMU, and workspace
distroforge init     # scaffold the workspace for the current chapter
distroforge build    # compile the Go init and pack the initramfs
distroforge run      # boot the image in QEMU
```

## Status

Working MVP. Chapters 00–05 are published, and the CLI builds and boots a real
system: `distroforge build && distroforge run` drops you into a login shell on
top of the Go init, all the way from an empty folder.

## License

[MIT](LICENSE)
