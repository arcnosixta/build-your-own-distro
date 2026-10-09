# Chapter 06 — A package manager in 500 lines

Every file in our image so far is there because we put it there by hand. That is
fine for a demo and useless for a distro: an operating system has to let you add
software later, remove it cleanly, and answer the question "what is installed?".
That is the entire job of a package manager, and it is smaller than the reputation
suggests.

## What a package manager actually is

Four responsibilities, in order of how essential they are:

| Piece | Job | Our answer |
|-------|-----|------------|
| **Format** | one file that carries a program and its metadata | `.pkg` = gzipped tar |
| **Database** | record what is installed, and which files belong to what | `/var/lib/pkg/db.json` |
| **Operations** | install, remove, list, inspect | `pkg install/remove/list/info` |
| **Trust & sourcing** | dependencies, signatures, repositories | *not yet* — see Chapter 09 |

APT, DNF, pacman and NixOS are all combinations of those four. We are going to
build the first three honestly, and be explicit about where the fourth one bites.

## The format: a gzipped tar

A package is a `.pkg`, which is a tar archive compressed with gzip — the exact
same container the kernel already gave us for the initramfs in Chapter 04. The
only rule is the layout inside it:

```
manifest.json          # name, version, description
files/                 # the payload, rooted at /
  usr/bin/hello
  usr/share/hello/message.txt
```

Why tar+gzip and not a custom format? Because it is streamable (we never need to
seek), universal (every language and OS can read it), and it is in the Go
standard library. The interesting design work is not the bytes on disk; it is
what we *store about* them.

## The database: remember what you own

When you install a package, the manager writes down which files it created:

```json
{
  "packages": {
    "hello": {
      "version": "1.0.0",
      "description": "demo package",
      "files": [
        "/usr/bin/hello",
        "/usr/share/hello/message.txt"
      ]
    }
  }
}
```

That `files` list is the whole trick. Without it, `remove` would have to guess
and would leave orphans; with it, removal is exact. A package that reinstalls over
an older version deletes the files the old version owned that the new one no
longer does — the same bookkeeping a real package manager does, minus the
transactions.

Two decisions worth noticing:

- **The manifest declares `depends`, but we do not resolve it yet.** The field is
  parsed and stored; enforcement (the topological install order, conflict
  detection, version ranges) is a Chapter 09 problem. Pretending otherwise would
  be dishonest about how deep dependency resolution goes.
- **File modes are inferred on the build host.** Windows has no executable bit, so
  `pkg build` marks anything under `bin/`, `sbin/`, `usr/bin/`, `usr/sbin/` as
  `0755` and everything else `0644` — the same portability hack Chapter 04 needed
  for `/sbin/init`.

## The tool

The whole package manager is `cmd/pkg/main.go` (about 440 lines, standard library
only):

```
pkg build   -name NAME -root DIR [-version V] [-desc TEXT] [-out FILE]
pkg install [-root DIR] FILE.pkg
pkg remove  [-root DIR] NAME
pkg list    [-root DIR]
pkg info    [-root DIR] NAME
```

The `-root` flag is the part that makes it a *build* tool as well as a runtime
tool: it installs into any directory, not just `/`. On the build host you point it
at `workspace/rootfs` to install software into the image offline; in the running
system you leave it at the default.

## Build a package

Make a tiny payload and pack it:

```bash
mkdir -p workspace/build/pkgtest/hello/usr/bin
mkdir -p workspace/build/pkgtest/hello/usr/share/hello

cat > workspace/build/pkgtest/hello/usr/bin/hello <<'EOF'
#!/bin/sh
echo Hello from the hello package, installed by pkg.
EOF
cat > workspace/build/pkgtest/hello/usr/share/hello/message.txt <<'EOF'
Minidistro package manager demo.
EOF

go run ./cmd/pkg build \
  -name hello -version 1.0.0 -desc "demo package" \
  -root workspace/build/pkgtest/hello \
  -out workspace/build/hello-1.0.0.pkg
```

```
built workspace/build/hello-1.0.0.pkg (hello 1.0.0, 7 entries, 0.4 KiB)
```

Seven entries: the manifest, four directories, and the two files. Now install it
into a scratch root and inspect the result:

```bash
go run ./cmd/pkg install -root workspace/build/fakeroot workspace/build/hello-1.0.0.pkg
go run ./cmd/pkg list   -root workspace/build/fakeroot
go run ./cmd/pkg info   -root workspace/build/fakeroot hello
```

```
installed hello 1.0.0 (2 files)
hello                1.0.0      demo package
name:    hello
version: 1.0.0
desc:    demo package
files:   2
  /usr/bin/hello
  /usr/share/hello/message.txt
```

Remove it and watch the files (`db.json` is the only thing left):

```bash
go run ./cmd/pkg remove -root workspace/build/fakeroot hello
```

```
removed hello 1.0.0 (2 files)
```

## pkg lives inside the image too

`distroforge build` now compiles `pkg` alongside `mininit` and installs both into
the rootfs:

```
stage 1/4: compile mininit (static linux/amd64)
        compiled workspace\build\mininit
stage 2/4: compile pkg (static linux/amd64)
        compiled workspace\build\pkg
stage 3/4: install binaries into rootfs
        installed rootfs/sbin/init and rootfs/sbin/pkg
stage 4/4: pack initramfs
        workspace/build/initramfs.cpio.gz: 37 entries, 4.9 MiB
```

Boot it (`distroforge run`) and you have the package manager at your prompt:

```
root@minidistro:~# /sbin/pkg list
no packages installed
```

To install something in the running system, get a `.pkg` there. The offline route
is easiest: copy it into the rootfs before building, then install it in the guest.

```bash
cp workspace/build/hello-1.0.0.pkg workspace/rootfs/root/
distroforge build && distroforge run
# in the guest:
/sbin/pkg install /root/hello-1.0.0.pkg
/root/usr/bin/hello
```

That last line is the point of the whole chapter: software that did not exist in
the image at build time, added to a running system, tracked in a database.

## Check yourself

1. Why does the package manager need to store the *list of files* of every
   installed package, instead of just its name and version?
2. The manifest has a `depends` field that we store but never read. What breaks
   the first time two packages need each other in a specific order, and what would
   you add to fix it?
3. Why can `pkg install` write into `workspace/rootfs` on your laptop and into
   `/` inside the VM with the same binary?

## Next

[Chapter 07 — Bootloader and UEFI](07-bootloader.md): the initramfs we build is
loaded straight by QEMU today. A real distro hands that job to a bootloader, and
that is the step that turns our rootfs into something a firmware can start.
