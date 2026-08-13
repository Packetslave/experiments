---
title: "How symbolic and hard links work on Unix"
date: 2026-08-13T00:38:00Z
slug: "symbolic-and-hard-links"
tags: ["unix", "linux", "macos", "filesystems"]
image: "images/posts/symbolic-and-hard-links.jpg"
imageCredit: "Photo: Brooklyn Museum Collection, no known copyright restrictions (via scikit-image)"
draft: false
---

Unix file systems support 2 kinds of link: hard links and symbolic links.
Both come from the same command, `ln`, but they work in very different ways.
This post explains how each kind works. It then covers where Linux and the
BSDs, including macOS, disagree.

## Inodes first

A file on disk is not really a name. It is an inode: a numbered record that
holds the file's metadata and points to its data blocks. The inode stores the
owner, the permissions, the timestamps, and the size. It does not store the
file name.

Names live in directories. A directory is a table that maps names to inode
numbers. Each mapping is a link. Run `ls -i` to see the inode number behind
each name.

## Hard links

A hard link is a second name for the same inode.

```console
$ echo hello > a.txt
$ ln a.txt b.txt
$ ls -li a.txt b.txt
1234567 -rw-r--r-- 2 brian staff 6 Aug 13 00:38 a.txt
1234567 -rw-r--r-- 2 brian staff 6 Aug 13 00:38 b.txt
```

Both names point to inode 1234567. Neither name is the original; the 2 names
are equal. The number after the permissions is the link count. It shows how
many names the inode has.

`rm` does not delete files. It removes a name and decreases the link count.
The kernel frees the data only when the count reaches 0 and no process holds
the file open. This is why you can delete a log file while a server writes to
it. The disk space comes back only when the server closes the file.

Hard links have 2 firm rules:

- They cannot cross file systems. An inode number only means something inside
  1 file system.
- You cannot make one to a directory. That could create loops that path
  resolution cannot handle.

## Symbolic links

A symbolic link (symlink) is a small file that contains a path, plus a flag
that tells the kernel to follow that path.

```console
$ ln -s a.txt c.txt
$ ls -l c.txt
lrwxrwxrwx 1 brian staff 5 Aug 13 00:39 c.txt -> a.txt
```

The kernel resolves the stored path each time you open the link. This makes
symlinks flexible. They can cross file systems, and they can point to
directories. It also makes them fragile. If you remove `a.txt`, the symlink
stays and points at nothing. This is a dangling link.

A relative target is resolved from the directory that holds the link, not
from your current directory. This is the most common way to create a dangling
link by accident.

## Which one to use

Use a symlink by default. It is visible in `ls`, it can point anywhere, and
you can see where it goes. Hard links are the right tool when many names must
share 1 file's storage. Backup tools such as `rsync --link-dest` use them to
build snapshots that share every unchanged file.

## Where Linux and BSD differ

The concepts above are identical everywhere. The tools and the edge cases are
not.

### The stat command

GNU and BSD `stat` share a name and almost nothing else. On Linux,
`stat -c '%i %h' file` prints the inode and link count. On macOS and FreeBSD,
the same query is `stat -f '%i %l' file`. Scripts that must run on both often
fall back to `ls -li`.

### Resolving a link chain

GNU coreutils has had `readlink -f` and `realpath` for decades. macOS added
both in macOS 12.3. On older versions you need Homebrew coreutils
(`greadlink -f`) or a language runtime.

### Replacing a symlink to a directory

`ln -sf new current` does the wrong thing when `current` is a symlink to a
directory: `ln` follows the old link and creates `new` inside the directory.
You must tell `ln` not to follow it. The flag is `-n` in GNU ln and `-h` in
BSD ln. Modern BSDs also accept `-n`, so `ln -sfn` is the portable spelling.
Deploy scripts that flip a `current` symlink hit this constantly.

### Symlink permissions

The mode on a symlink (`lrwxrwxrwx`) is decoration on Linux. The kernel
ignores it, and `chmod` on a symlink changes the target instead. BSD kernels
have `lchmod`, so on macOS `chmod -h` changes the link's own mode. Every
system still ignores that mode when it follows the link. This mostly matters
to archive tools that restore modes exactly.

### Hard links to directories

Linux has never allowed them. Apple's old HFS+ file system did, as a private
feature, and Time Machine was built on it: an unchanged directory in each
backup was a hard link to the same directory in the previous backup. APFS
dropped the feature and uses file system snapshots instead.

## 2 macOS extras

**Firmlinks.** Since macOS Catalina, the system volume is read-only. A fixed
set of firmlinks grafts it onto the writable data volume, so `/Applications`
and `/Users` feel like 1 tree. The pairs are listed in
`/usr/share/firmlinks`. You cannot create your own.

**Finder aliases are not symlinks.** An alias is a Finder-level bookmark that
tracks its target even when the target moves. The shell sees it as an opaque
data file, and `cd` into one fails. If a command line tool needs to follow
it, make a symlink instead.

## The short version

A hard link is another name for the same inode. A symlink is a file that
contains a path. Almost everything else follows from those 2 sentences.
