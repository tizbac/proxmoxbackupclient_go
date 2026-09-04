# WinPE Rescue ISO Builder

Builds a bootable Windows PE rescue ISO that bundles the Proxmox Backup
Client GUI and auto-launches it at start.

## Requirements (Linux host)

- `p7zip-full`  — ISO extraction
- `wimtools`    — `wimlib-imagex` WIM read/modify
- `xorriso`     — bootable ISO rebuild

## Building the GUI exe first

```sh
./build_gui_windows_docker.sh        # -> ProxmoxBackupClientGO.exe
```

## Usage

```sh
./build_winpe_iso.sh -i <any-winpe.iso> -x <app.exe> -o <output.iso>
```

Example with a downloaded WinPE ISO:

```sh
curl -C - -o WinPE_base.iso \
  https://archive.org/download/WinPE-10.0.1904/WinPE_amd64_v10.0.19041.1.iso
./build_winpe_iso.sh -i WinPE_base.iso -x ProxmoxBackupClientGO.exe
```

### Options

| Flag | Meaning |
|------|---------|
| `-i <iso>`   | Base WinPE ISO (default `WinPE_base.iso` in CWD) |
| `-x <exe>`   | Windows exe to embed (default `ProxmoxBackupClientGO.exe`) |
| `-o <iso>`   | Output ISO path (default `ProxmoxBackupClient-WinPE.iso`) |
| `-a <cmd>`   | Custom launch script placed at `X:\Start.cmd` |
| `-k`         | Keep intermediate work dir (deleted on success by default) |
| `-h`         | Help |

## How it works

1. Extracts the source ISO with `7z`.
2. Injects `<app>.exe`, a `Start.cmd` launcher, and a rewritten
   `startnet.cmd` into `sources/boot.wim` (image 1) via `wimlib-imagex update`.
3. Locates and extracts the hidden El-Torito BIOS (`etfsboot.com`) and
   UEFI (`efisys.bin`) boot images.
4. Rebuilds the ISO with `xorriso -as mkisofs`, preserving both boot records.

At boot, WinPE runs `wpeinit`, then our `startnet.cmd` finds `Start.cmd` on
the boot media, copies the exe to the RAM drive and launches it.

## Test

```sh
qemu-system-x86_64 -cdrom ProxmoxBackupClient-WinPE.iso -m 2048 -boot d
```

## Caveats

- The Wails GUI (`ProxmoxBackupClientGO.exe`) needs a desktop session and
  WebView2 runtime, both of which a bare WinPE lacks. The GUI may therefore
  fail to launch in a plain WinPE shell; `Start.cmd` drops to a rescue shell
  if that happens. Customize `Start.cmd` (`-a`) for your environment needs.
- The built ISO is ~620 MB and is not tracked by git (only `build_winpe_iso.sh`
  and this README are committed).
