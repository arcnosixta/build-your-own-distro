# Chapter 01 — Your build environment

Building a distro is mostly file manipulation, compiling, and booting the
result in a virtual machine. This chapter sets up a workspace that keeps
those three activities cleanly separated — the same layout LFS uses, for the
same reason: builds must be repeatable from a clean tree.

## Requirements

- **A build shell**: Linux (native or WSL2) is recommended, because the chapter
  commands use a Unix shell and standard tools
- **Windows without WSL2**: also supported — `distroforge build` and
  `distroforge run` work natively using a prebuilt kernel (Chapter 03, Option C)
- **QEMU**: `qemu-system-x86_64` for booting what we build
- **Toolchain**: `gcc`, `make`, `binutils` — **only** needed if you build the
  kernel from source (Chapter 03, Option D)
- **Disk**: ~40 GB free if you compile a kernel; a few hundred MB if you borrow
  kernels and binaries
- **RAM**: 4 GB minimum for kernel builds, 8 GB comfortable

No `cpio` is required: `distroforge` packs the initramfs itself.

## Install the tools

On Debian/Ubuntu/WSL2:

```bash
sudo apt update
sudo apt install -y build-essential qemu-system-x86 qemu-utils \
  xorriso grub-pc-bin grub-efi-amd64-bin mtools dosfstools
```

On Arch:

```bash
sudo pacman -S --needed base-devel qemu-system-x86 xorriso \
  grub mtools dosfstools
```

On Windows (native, no WSL2):

```powershell
winget install --id GoLang.Go -e
winget install --id SoftwareFreedomConservancy.QEMU -e
```

QEMU installs to `C:\Program Files\qemu\` and is not always added to `PATH`; the
CLI looks there for it automatically.

Verify everything at once:

```bash
distroforge check
```

Expected output is one `[ ok ]` line per tool. Fix anything marked `FAIL` before
continuing. Tools marked as optional (the compiler, `xorriso`, `grub-mkrescue`)
are only needed by later chapters — the CLI tells you which.

### WSL2 notes

- Enable nested virtualization for KVM: in `%UserProfile%\.wslconfig` set
  `[wsl2]`, `nestedVirtualization=true`, then `wsl --shutdown`.
- Keep the workspace inside the Linux filesystem (`~/...`), **not** under
  `/mnt/c/` — cross-filesystem builds are an order of magnitude slower and
  break permissions.

### Windows-native notes

- The CLI itself works identically on Windows; only the *chapter shell
  commands* (e.g. `apt download`, `ln -s`) assume Linux.
- Because Windows cannot create the busybox symlinks Linux distros rely on, the
  init falls back to `/bin/busybox sh` (Chapter 04). Same shell, same result.
- Prefer a Linux shell for Chapters 05 and up, where we start compiling and
  installing packages into the rootfs.

## The workspace layout

```bash
distroforge init
```

creates:

```
workspace/
├── sources/     # downloaded tarballs, kept between chapters
├── build/       # compile trees, disposable
├── rootfs/      # the future root filesystem of our distro
└── iso/         # final bootable image
```

Three rules, borrowed from LFS's hard-won experience:

1. **Never build as root.** Only the final installation steps get `sudo`.
2. **`build/` is disposable.** If something smells wrong, delete it and rerun.
3. **`sources/` survives.** Kernels are 140 MB; don't redownload them.

## QEMU smoke test

Before building anything, confirm QEMU itself works with its built-in test
image:

```bash
qemu-system-x86_64 -machine accel=kvm -m 512 -nographic -kernel /dev/null 2>&1 | head -1
```

You should see QEMU complain about the kernel, not about missing KVM or
missing binary. On bare metal, `accel=kvm` needs `/dev/kvm` accessible; in
WSL2 with nested virt enabled it works out of the box.

## Check yourself

1. Why must `sources/` and `build/` be separate directories?
2. What does `-nographic` change about QEMU's behavior?
3. Why is running the build as root dangerous beyond "you might delete `/`"?

## Next

[Chapter 02 — The root filesystem](02-rootfs.md): create the skeleton that
will become `/` of our distribution.
