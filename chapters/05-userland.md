# Chapter 05 — Userland: from busybox to coreutils

Our system boots and gives you a prompt, but the prompt is connected to exactly
one program: busybox. Everything a user *does* — listing files, editing configs,
logging in — lives in the userland layer. This chapter makes that layer real:
startup files, a login shell, and the story of how to replace one static
busybox with a full GNU userland.

## What "userland" contains

Four things, in the order they matter at boot:

| Piece | Examples | Job |
|-------|----------|-----|
| Shell | `sh`, `bash`, `zsh` | Read and run commands |
| Coreutils | `ls`, `cat`, `cp`, `grep` | The verbs of the OS |
| Libraries | glibc, musl | Code shared by those programs |
| Startup files | `/etc/profile`, `~/.profile` | Configure each shell |

A first-process init and a shell is the minimum. A *distro* adds the rest so the
shell is usable by a person.

## The startup files, and their order

A **login shell** reads configuration so every session starts consistently.
Busybox `ash` (our `sh`) follows the classic order:

```
login shell starts
  → /etc/profile          (system-wide, read once)
      → . /etc/os-release (identify the distro)
      → export PATH       (where to find commands)
      → export PS1        (the prompt)
      → print /etc/motd   (the welcome banner)
  → ~/.profile            (per-user, read after)
```

`distroforge init` now scaffolds these. Look at them:

```bash
cat workspace/rootfs/etc/profile
cat workspace/rootfs/etc/os-release
cat workspace/rootfs/etc/motd
```

`/etc/os-release` is the file every tool from `lsb_release` to `neofetch` reads
to name your distribution. Until now your system had no name; this is where it
gets one.

### A bug worth feeling: reading `/etc/profile` twice

Our first `~/.profile` re-sourced `/etc/profile` "to be safe":

```sh
# WRONG — the welcome banner printed twice
[ -r /etc/profile ] && . /etc/profile
```

The login shell already reads `/etc/profile` *before* `~/.profile`. Sourcing it
again ran the whole thing twice — a doubled MOTD. This is the single most common
shell-startup mistake: startup files are **layered, not chained**. `~/.profile`
adds to `/etc/profile`; it does not re-run it.

## busybox applets: one binary, many names

`busybox` is not one program — it is hundreds, selected by the name it is invoked
as. When busybox starts, it looks at `argv[0]`:

```
/bin/ls     → busybox runs its "ls"  applet
/bin/cat    → busybox runs its "cat" applet
/bin/sh     → busybox runs its "ash" applet
```

Those names are normally **symlinks** into the one binary. On a Linux shell:

```bash
cd workspace/rootfs/bin
for c in sh ls cat cp mv rm grep sed awk mount umount echo \
         mkdir rmdir ps kill sleep ln chmod chown find; do
  ln -sf busybox "$c"
done
```

`busybox --install -s /bin` does the same for the full applet list.

On Windows you cannot create symlinks, so our init shell falls back to
`/bin/busybox sh` — the shell works, but `cat` and friends do not exist, which is
why the chapter commands that `cat` a file assume a Linux shell. This is not a
quirk of our project; it is the applet mechanism exposed.

## Static vs dynamic: the library question

busybox is **statically linked**: all the code it needs is inside the file. Most
distro binaries are **dynamically linked** — they load library code from
`/lib`/`/usr/lib` at runtime:

```bash
ldd /bin/ls        # on a real distro: lists libc.so, libselinux.so, ...
```

That is why a "small" binary needs a whole `/lib` tree, and why a missing or
mismatched `libc.so` is the classic "cannot execute: No such file or directory"
error — the *loader* is missing, not the program. Our `mininit` and busybox are
static precisely so the system can boot with an empty `/lib`.

## Replacing busybox with a real userland (Linux)

busybox trades features for size. To get the full GNU tools, on a Linux shell you
extract real packages into the rootfs and drop busybox symlinks in their favor:

```bash
cd workspace/sources
apt download coreutils dash grep sed
for deb in *.deb; do dpkg-deb -x "$deb" ../rootfs/; done
```

Then rebuild and boot: `cat` now resolves to GNU coreutils, and `cat --version`
reports GNU, not busybox. The rootfs grew from 1 MB to tens of MB — that
trade-off *is* the difference between embedded and desktop distros.

For a from-source userland you would build musl + coreutils against it, which is
where LFS spends most of its pages. We deliberately stop at "use the packages"
and save the compilation tax for the tools you actually want to build yourself.

## Boot it and see the results

```bash
distroforge build
distroforge run
```

After the mount lines, your now-named distro greets you:

```
mininit: started shell (pid 433)
Welcome to Minidistro — built from scratch.
Edit /etc/profile to customize your shell.
root@minidistro:~#
```

That prompt is built from `/etc/profile`; the banner from `/etc/motd`; the name
`minidistro` from `/etc/os-release`. Change any of those files, `distroforge
build`, and boot again — your distro's personality is now data, not code.

## Check yourself

1. Why does a login shell read `/etc/profile` before `~/.profile`, and what
   breaks if the second re-sources the first?
2. What does `argv[0]` have to do with busybox providing hundreds of commands
   from one binary?
3. A binary fails with "No such file or directory" but `ls -l` shows it exists.
   What is actually missing, and how does static linking avoid it?

## Next

[Chapter 06 — A package manager in 500 lines](06-packages.md): track what is
installed, install more, and manage a database of files.
