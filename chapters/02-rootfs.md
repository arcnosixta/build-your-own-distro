# Chapter 02 — The root filesystem

The root filesystem is the contract of a distribution: a fixed directory
tree where every program knows to look for its config, its libraries, and its
devices. This chapter builds that skeleton by hand — after it, no directory
on a Linux system will ever look arbitrary to you.

## The Filesystem Hierarchy in one screen

```
rootfs/
├── bin, sbin   # essential executables (merged into /usr on modern distros)
├── etc         # configuration — the distro's personality
├── lib         # shared libraries
├── proc, sys   # kernel interfaces (mounted at boot, empty in our tree)
├── dev         # device nodes (created by the kernel at boot)
├── tmp, var    # scratch and persistent state
├── home        # users
├── root        # root user's home
├── boot        # kernel + bootloader files, mounted separately in real distros
└── usr         # everything non-essential: apps, docs, headers
```

The distinction that confuses newcomers: `proc` and `sys` are **not folders
you fill** — they are mount points where the kernel publishes its own state.
Our tree contains them empty; the kernel populates them at boot.

## Build the skeleton

```bash
distroforge init
cd workspace/rootfs
```

The command creates every directory above with correct permissions
(`tmp` and `run` are mode 1777 — the sticky bit matters). Then create the
three files every Unix refuses to run without:

```bash
cat > etc/passwd << 'EOF'
root:x:0:0:root:/root:/bin/sh
EOF

cat > etc/group << 'EOF'
root:x:0:
EOF

cat > etc/fstab << 'EOF'
# <fs>      <mountpoint>  <type>  <options>  <dump> <pass>
proc        /proc         proc    defaults   0      0
sysfs       /sys          sysfs   defaults   0      0
tmpfs       /run          tmpfs   mode=0755  0      0
EOF
```

- `passwd` maps names to UIDs — the kernel only ever sees numbers.
- `group` does the same for groups.
- `fstab` is the boot-time mount table; ours declares the pseudo-filesystems.

## Prove it with a chroot

You can already "enter" this filesystem to feel how thin it is:

```bash
sudo chroot workspace/rootfs /bin/sh
```

It fails: there is no `/bin/sh` yet. That failure **is** the lesson — a
rootfs skeleton is structure, not software. Chapters 05–06 fill it with
busybox and our package manager; until then, `chroot` failing with
"no such file or directory" (which actually means "missing loader or binary")
is your progress marker.

A working check for now:

```bash
# from outside the chroot
find workspace/rootfs -type d | sort   # the full skeleton
```

## Why permissions are part of the contract

| Path | Mode | Why |
|------|------|-----|
| `/tmp` | 1777 | world-writable, sticky — anyone can create, only owner deletes |
| `/root` | 700 | root's home must not be readable |
| `/etc/shadow` (later) | 640 | password hashes, group `shadow` only |
| `/run` | 755 | runtime state, cleared every boot |

If you get these wrong, the distro *boots* but is broken in ways that take
weeks to diagnose. Set them once, correctly, here.

## Check yourself

1. Why does `rootfs/proc` contain nothing after `distroforge init`?
2. What breaks if `/tmp` loses its sticky bit?
3. Modern distros merge `/bin` into `/usr/bin`. What must change in the
   bootloader for that to work?

## Next

[Chapter 03 — Kernel: build it or borrow it](03-kernel.md)
