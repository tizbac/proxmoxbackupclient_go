# elastio-snap installer

The snapshot module installer the app's **Install it for me** and
`salvanza-cli install-snapshot-module` run on Linux. The .deb and .rpm install this
folder as `/usr/share/salvanza/elastio-snap/`.

| File | What it is |
|---|---|
| `install-elastio-snap.sh` | the installer: build tools and kernel headers, DKMS build, Secure Boot key enrolment, loading at every start (its header says how) |
| `kernel7-strscpy.patch` | the project's kernel 7 fix, offered to Axcient as [pull request 138](https://github.com/Axcient/elastio-snap/pull/138) |
| `source/` | Axcient's elastio-snap, unchanged, at the tested commit |

## source/

Axcient/elastio-snap, branch `develop`, commit `971ba56a607022efc72cb3567c3cc29b2e4d10cf`
(version 0.12.11), [github.com/Axcient/elastio-snap](https://github.com/Axcient/elastio-snap).
Only what the module's DKMS build needs, byte for byte as published (`.gitattributes`
keeps git from changing line ends): `src/`, `dist/elastio-snap-dkms-conf`, and the
licence files `COPYING` and `LICENSING.md`, with `README.md`. The kernel module is
GPL-2.0-only (Copyright 2015 Datto, Inc.; 2020 Elastio Software Inc.), unlike the rest
of this project (GPL-3.0-or-later); it is shipped as source and built on the user's
computer.

To move to another commit, export the same files from it and replace `source/`, then
change `PINNED_REF` in the installer, which also names the commit in its notes:

```bash
git clone https://github.com/Axcient/elastio-snap.git /tmp/elastio-snap
git -C /tmp/elastio-snap archive --format=tar <commit> src dist/elastio-snap-dkms-conf COPYING LICENSING.md README.md \
  | tar -x -C packaging/linux/elastio-snap/source
```

The installer checks the patch against the new source: when the source already has the
fix it skips the patch, and when the patch no longer fits it stops before building.
