# Salvanza GUI: user manual

This manual covers the graphical client (the GUI) of Salvanza for Proxmox Backup Server, formerly Proxmox Backup Client, on Windows and Linux. It applies to version 2.0 (see the [changelog](../../CHANGELOG.md)).

For the command line, `salvanza-cli`, see [CLI.md](CLI.md). For restoring a whole machine onto bare metal see [BMR.md](BMR.md).

## Contents

1. [What the GUI does](#1-what-the-gui-does)
2. [Install and start](#2-install-and-start)
3. [The window](#3-the-window)
4. [Adding a PBS server](#4-adding-a-pbs-server)
5. [One-off backups](#5-one-off-backups)
6. [Backup Sets](#6-backup-sets)
7. [Incremental folder backups (change detection)](#7-incremental-folder-backups-change-detection)
8. [The backup progress card](#8-the-backup-progress-card)
9. [Restore](#9-restore)
10. [Undelete](#10-undelete)
11. [Roll back](#11-roll-back)
12. [Reports](#12-reports)
13. [Message Log and log files](#13-message-log-and-log-files)
14. [Preferences](#14-preferences)
15. [Background service, tray and closing the window](#15-background-service-tray-and-closing-the-window)
16. [Compatibility with the official Proxmox client](#16-compatibility-with-the-official-proxmox-client)
17. [Troubleshooting](#17-troubleshooting)

## 1. What the GUI does

The GUI backs up Windows and Linux computers to a Proxmox Backup Server (PBS) and restores from it.

* **Folder backups** store one or more folders as a file archive (pxar). You can browse them and restore single files.
* **Machine backups** store whole physical disks as images. You restore them onto bare metal with the Clonezilla ISO ([BMR.md](BMR.md)), or onto Proxmox VE if you stored them as a `vm` snapshot. You can also browse a machine backup in Restore and take single files out of it ([section 9.6](#96-files-from-a-machine-backup)).
* **Backup Sets** are named, saved jobs with a schedule. **One-off backups** run once and are not saved.
* **Change detection** (Metadata mode) makes repeat folder backups read only new and changed files. See [section 7](#7-incremental-folder-backups-change-detection).
* **Undelete** finds files that have gone from a Backup Set's folders and puts them back. **Roll back** returns a set's folders to how they were at a chosen backup, with a preview first and Undo afterwards. Both work for folder and machine sets.
* **Client-side encryption** encrypts every chunk before it leaves the machine, using a standard PBS key file.
* **A background service** (Windows and Linux) runs the schedules when nobody is signed in and the GUI is closed.

## 2. Install and start

### Windows

Each release on the [releases page](https://github.com/salvanza/salvanza_client/releases) has the installer, `Salvanza-<version>-x64.msi` (a branded build has its own, such as `NimbusBackup-<version>-x64.msi`). Check it against `SHA256SUMS.txt`:

```powershell
Get-FileHash .\<downloaded file> -Algorithm SHA256
```

The installer installs the app (`salvanza-gui.exe`), its background service (`salvanza-svc.exe`) and the command line (`salvanza-cli.exe`, [CLI.md](CLI.md)) under `C:\Program Files\Salvanza\`, adds that folder to the system `PATH`, installs the service as `salvanza-svc` (display name "Salvanza for Proxmox Backup Server Service") and starts it, and adds a Start menu entry and an entry in Programs and Features. Its **Start Salvanza for Proxmox Backup Server when a user signs in** tick box, after the installation folder, starts the app in the tray for every user who signs in; it is unticked unless you turned it on in an earlier install, and the service runs the Backup Sets either way. Running a newer MSI upgrades in place and keeps that choice. To remove it and choose whether to keep the settings (servers and tokens, Backup Sets, keys, reports and logs), use **Change** in Programs and Features (**Modify** in Windows 11 Settings > Apps), then **Remove**; plain **Uninstall** keeps them without asking.

**Upgrading from Proxmox Backup Client.** Salvanza is the new name of Proxmox Backup Client. Its installer removes an installed Proxmox Backup Client (or another brand's build, such as Nimbus Backup) as it would an older version of itself, closes its app and removes its service, `ProxmoxBackupClientSVC`. When the new service first starts, it moves the earlier settings, `C:\ProgramData\ProxmoxBackupClient`, to `C:\ProgramData\Salvanza`, so the PBS servers, keys, Backup Sets and history carry on, and keeps a copy of the settings files in `C:\ProgramData\Salvanza\pre-salvanza-<date>`. The app does the same for the folders of each user (`%APPDATA%\ProxmoxBackupClient`, `%LOCALAPPDATA%\ProxmoxBackupClient` and `%USERPROFILE%\.proxmox-backup-guardian`) the first time that user starts it. Nothing is moved when the new folder already has settings. The start-at-sign-in choice carries over too.

Start **Salvanza for Proxmox Backup Server** (`salvanza-gui.exe`). Without the service, start it with **Run as administrator**: administrator rights are needed for VSS (consistent snapshots of files that are open) and for machine backups. With the service running, backups go through the service, which has those rights, and the app asks for administrator rights once to connect to it ([section 15](#connecting-to-the-background-service)).

The builds are not code signed. Windows SmartScreen or Defender may warn about an unknown or machine-learning flagged program (`!ml`). This is a known false positive for unsigned Go programs. Verify the hash instead.

Only one copy of the GUI runs at a time on Windows. Starting it again brings the running copy forward.

### Linux

* **Packages**: `.deb` (Debian, Ubuntu, Mint), `.rpm` (Fedora) and an Arch package (built from [packaging/arch/PKGBUILD](../../packaging/arch/PKGBUILD) with `make pkgarch`), package name `salvanza`. They install the app in `/usr/lib/salvanza/` (started by `/usr/bin/salvanza-gui`), the command line `salvanza-cli` ([CLI.md](CLI.md)), the manuals, the background service (turned on by a fresh install) and one menu entry, **Salvanza for Proxmox Backup Server**, which starts the app as you. With the service it asks for your password once to connect to it ([section 15](#connecting-to-the-background-service)). For the few things that need the app itself as root (machine backups without the service, restoring into folders only root may write), the app offers **Restart as administrator**, which runs `salvanza-gui-root` (pkexec from the desktop, sudo from a terminal). The `salvanza` package replaces the earlier `proxmoxbackupclient` (and `pbsgo`) package; the service moves the earlier settings, `/var/lib/proxmoxbackupclient`, to `/var/lib/salvanza` the first time it starts, and the app moves each user's `~/.proxmox-backup-guardian` to `~/.salvanza`, keeping a copy of the settings files in a `pre-salvanza-<date>` folder there.

The app needs the WebKitGTK runtime that Wails programs use; most desktop distributions already have it. Folder backups run as a normal user. **Machine** backups need root. Linux has no VSS: a consistent snapshot of a mounted disk needs the `elastio-snap` (or `dattobd`) kernel module loaded and root rights. Installing the module is described next.

### Linux snapshots: installing elastio-snap or dattobd

A machine backup of a running Linux computer reads each mounted partition through a snapshot, as Windows does with VSS, so the image is consistent. Linux has nothing built in for this, so the app uses a kernel module: `elastio-snap` or `dattobd` (elastio-snap is a fork of dattobd). A folder backup uses the same snapshot when its Backup Set has **Use VSS (Windows Shadow Copy / Linux Snapshot)** ticked, and so needs the module and root too; unticked, it reads the folders as they are. Neither module can snapshot **btrfs**, the default file system of Fedora: there, untick it for folder sets, and a machine set needs **Use snapshots** off (with its acceptance box) for its btrfs partitions.

A machine backup needs three things:

1. **One of the modules loaded.** If both are loaded, the app uses elastio-snap.
2. **Root rights.** The background service has them. Without it, use **Restart as administrator**, or run `salvanza-cli backup-disk` with `sudo`.
3. **At least 10% of each snapshotted file system free.** The modules keep the snapshot's copy-on-write file on the file system they snapshot, and with less room taking the snapshot fails (or the file fills up during the backup). The run stops before it starts, saying which volume is too full, how much is free and how much more is needed. The same check applies to a folder backup that uses a snapshot, and to `salvanza-cli backup-folder` and `backup-disk`.

**Install the module through DKMS** (Dynamic Kernel Module Support). A kernel module is built for one kernel. DKMS builds it again for each new kernel when the kernel is updated, so the module keeps loading. A module built with plain `make install` is left behind by the next kernel update, and machine backups stop until it is built again.

**Load it at every start.** A backup run as root loads an installed module by itself, but the app in your own window cannot, so the machine backup screen says no module is loaded until something has loaded it. The steps below add a file in `/etc/modules-load.d/` for this.

#### Install it from the app

The easiest way, on Debian, Ubuntu, Linux Mint, Fedora, the RHEL family and Arch Linux: when no module is loaded, the machine backup screen's yellow box has **Install it for me**. It runs the installer the `.deb`, `.rpm` and Arch packages bring, in `/usr/share/salvanza/elastio-snap/`, which builds elastio-snap from the source packaged with it ([the project's script](#elastio-snap-with-the-projects-script), below), loads it, and loads it at every start. It asks for an administrator's password, installs the build tools and kernel headers with apt, dnf or pacman (a download of a few hundred MB, so it needs the internet), and takes a few minutes; its output shows as it runs. When it ends, the box says **Snapshot module loaded: elastio-snap**.

With **Secure Boot** on, the kernel loads the module only once its signing key is approved, which you do once, at the next restart. The app asks you first for a one-time password of 8 to 16 letters and digits, then:

1. Restart the computer.
2. A blue screen, **Perform MOK management**, appears: press a key within 10 seconds (if you miss it, the computer starts as usual with nothing enrolled; use **Install it for me** again to ask again).
3. Choose **Enroll MOK**, then **Continue**, then **Yes**.
4. Type the one-time password, then choose **Reboot**. The blue screen may read the keyboard as an American (US) one.

Until that restart the box gives these steps instead of the button. From a terminal, `sudo salvanza-cli install-snapshot-module` does the same ([CLI.md](CLI.md#installing-the-snapshot-module-linux)).

The approval screen is shim's, so this works on computers that start through shim, as Ubuntu, Debian, Linux Mint and Fedora do. Arch Linux with Secure Boot usually starts with the owner's own keys instead (`sbctl`): there the installer stops before building anything and says so. Sign the module with your own key, or turn Secure Boot off.

On Arch Linux, pacman installs only what is missing and does not refresh its package lists (Arch does not support that without updating the whole system). If it cannot find a package, or the computer runs a kernel an update has since replaced, the installer says so: update with `sudo pacman -Syu`, restart, and try again.

What to install by hand (tested in November 2026):

| Distribution | Install | Tested |
|---|---|---|
| Ubuntu, Linux Mint, Debian | dattobd from Datto's packages ([below](#dattobd-from-datto-packages-debian-ubuntu-mint)), or elastio-snap with the project's script ([below](#elastio-snap-with-the-projects-script)) | Both, on Linux Mint 21.3 (Ubuntu 22.04 base) with kernel 5.15: dattobd 0.12.2, and elastio-snap 0.12.11 by hand and with the script. An earlier test built and ran elastio-snap 0.12.11 on Debian 13 with kernel 6.12 |
| Fedora | elastio-snap with the project's script ([below](#elastio-snap-with-the-projects-script)) | Fedora 44 with kernel 7.2, the kernel it has after its updates: the script's patched build loads at every start and takes snapshots. Unpatched elastio-snap builds only on Fedora 44's release kernel, 6.19 ([kernel 7](#kernel-7)) |
| RHEL, AlmaLinux, Rocky Linux 8 to 10 | dattobd from Datto's packages, or elastio-snap with the project's script | Not tested |
| Arch Linux, Manjaro, EndeavourOS | elastio-snap with the project's script ([below](#elastio-snap-with-the-projects-script)) | Arch Linux with kernel 7.2: the script's patched build of elastio-snap 0.12.11, started from **Install it for me**, loads at every start and takes snapshots |

Elastio's own package repositories, named in elastio-snap's INSTALL.md, no longer answer, and no repository has an `elastio-snap-dkms` package (the app's .deb suggests Datto's `dattobd-dkms` instead). elastio-snap is built from source: the project's script does it, and the steps by hand follow it. Use the maintained fork, [Axcient/elastio-snap](https://github.com/Axcient/elastio-snap) (branch `develop`): the original `elastio/elastio-snap` is archived and does not build on current kernels.

#### elastio-snap with the project's script

The project's script, [packaging/linux/elastio-snap/install-elastio-snap.sh](../../packaging/linux/elastio-snap/install-elastio-snap.sh), installs elastio-snap on Debian, Ubuntu, Linux Mint, Fedora, the RHEL family and Arch Linux, also on kernel 7, which elastio-snap does not support yet ([kernel 7](#kernel-7)). It is what **Install it for me** runs. The `.deb`, `.rpm` and Arch packages install it in `/usr/share/salvanza/elastio-snap/`; without them, take it from the repository:

```bash
sudo /usr/share/salvanza/elastio-snap/install-elastio-snap.sh
# or, without the package:
git clone --depth 1 https://github.com/salvanza/salvanza_client.git
sudo salvanza_client/packaging/linux/elastio-snap/install-elastio-snap.sh
```

The script:

1. Takes the elastio-snap source in `source/` beside it: Axcient's code at the commit the project tested (0.12.11, branch `develop`, November 2026), unchanged, under its own licence (GPL-2.0).
2. Applies the project's kernel 7 patch, `kernel7-strscpy.patch` next to the script. The patched module also works on older kernels, and keeps building when a kernel 7 update arrives.
3. Installs the compiler, DKMS and the running kernel's headers with apt, dnf or pacman, and the headers meta-package (`linux-headers-generic`, `linux-headers-amd64` or `kernel-devel`) so DKMS has the headers of each new kernel. On Arch it installs the headers of every installed kernel (`linux-headers`, `linux-lts-headers` and so on), so DKMS rebuilds the module whichever kernel an update brings.
4. Builds and installs it through DKMS.
5. With Secure Boot on, finds the key DKMS signed the module with and, if it is not enrolled yet, asks the firmware to enrol it at the next start (`mokutil --import`, with a one-time password: asked on the terminal, or read from standard input with `--mok-password-stdin`). It then ends with exit code 10 and the restart steps ([Secure Boot](#secure-boot)).
6. Loads it, and adds `/etc/modules-load.d/elastio-snap.conf`.

Running it again changes nothing when that build is already installed. If an older elastio-snap is loaded and no snapshot is in use, the script loads the new build in its place; with a snapshot in use, the new build loads at the next start. `--ref <commit, tag or branch>` fetches and builds another elastio-snap version with git instead of the packaged source, and `--with-elioctl` (with `--ref`) also installs the module's own tool, `elioctl`, in `/usr/local/bin`. The script checks the patch against that version first: when it already has the fix, it builds without the patch; when the patch no longer fits Axcient's code, it stops before changing anything and says so, rather than build a half-patched module. The patch is offered to Axcient as [pull request 138](https://github.com/Axcient/elastio-snap/pull/138); once Axcient releases the fix, the project drops the patch.

The sections below give the same steps by hand.

#### dattobd from Datto packages (Debian, Ubuntu, Mint)

Datto publishes dattobd for Ubuntu LTS releases and for Debian (its repository has `jammy`, `noble`, `bookworm` and `trixie`, among others). The packages build the module with DKMS and load it at every start (`/etc/modules-load.d/dattobd.conf`). Linux Mint needs the name of the Ubuntu release it is based on, which `/etc/os-release` gives as `UBUNTU_CODENAME`; `lsb_release -sc` gives Mint's own name (such as `virginia`), which Datto does not have.

```bash
. /etc/os-release
CODENAME=${UBUNTU_CODENAME:-$VERSION_CODENAME}
curl -fsSL https://cpkg.datto.com/DATTO-PKGS-GPG-KEY | sudo gpg --dearmor -o /usr/share/keyrings/datto-archive-keyring.gpg
echo "deb [arch=amd64 signed-by=/usr/share/keyrings/datto-archive-keyring.gpg] https://cpkg.datto.com/datto-deb/public/$CODENAME $CODENAME main" | sudo tee /etc/apt/sources.list.d/datto-linux-agent.list
sudo apt-get update
sudo apt-get install dattobd-dkms dattobd-utils
sudo modprobe dattobd
```

Datto's own [INSTALL.md](https://github.com/datto/dattobd/blob/main/INSTALL.md) adds the key with `apt-key`, which is deprecated; the `signed-by` form above does the same. `dattobd-utils` adds `dbdctl`, the module's own tool. The app does not need it.

#### elastio-snap with DKMS (Debian, Ubuntu, Mint)

By hand, without the kernel 7 patch (for that, see [kernel 7](#kernel-7)):

```bash
sudo apt-get install git build-essential dkms linux-headers-$(uname -r)
git clone --depth 1 https://github.com/Axcient/elastio-snap.git
cd elastio-snap
VER=$(grep -oP 'ELASTIO_SNAP_VERSION "\K[^"]+' src/elastio-snap.h)
sudo cp -r src /usr/src/elastio-snap-$VER
sed "s/@MODULE_VERSION@/$VER/" dist/elastio-snap-dkms-conf | sudo tee /usr/src/elastio-snap-$VER/dkms.conf > /dev/null
sudo dkms install elastio-snap/$VER
echo elastio-snap | sudo tee /etc/modules-load.d/elastio-snap.conf
sudo modprobe elastio-snap
```

`dkms install` builds the module for the running kernel and installs it in `/lib/modules/<kernel>/updates/dkms/`. When a kernel update is installed, DKMS builds it for the new kernel by itself. For that it needs the new kernel's headers, so keep the headers meta-package installed (`linux-headers-generic` on Ubuntu and Mint, `linux-headers-amd64` on Debian).

The module's own tool, `elioctl`, is optional. To build it, in the same folder:

```bash
sudo make application-static
sudo install -m 0755 app/elioctl /usr/local/bin/elioctl
```

Do not run `sudo make install`: it also installs a copy of the module outside DKMS, which the next kernel update leaves behind. If an older version was installed that way, remove it first: `sudo make uninstall` in its source folder, then `sudo depmod -a`.

#### Fedora and the RHEL family

By hand, without the kernel 7 patch, which Fedora 44 needs ([kernel 7](#kernel-7)):

```bash
sudo dnf install git gcc make dkms kernel-devel-$(uname -r) kernel-devel
git clone --depth 1 https://github.com/Axcient/elastio-snap.git
cd elastio-snap
VER=$(grep -oP 'ELASTIO_SNAP_VERSION "\K[^"]+' src/elastio-snap.h)
sudo cp -r src /usr/src/elastio-snap-$VER
sed "s/@MODULE_VERSION@/$VER/" dist/elastio-snap-dkms-conf | sudo tee /usr/src/elastio-snap-$VER/dkms.conf > /dev/null
sudo dkms install elastio-snap/$VER
echo elastio-snap | sudo tee /etc/modules-load.d/elastio-snap.conf
sudo modprobe elastio-snap
```

`kernel-devel` keeps the headers of each new kernel coming, which DKMS needs to rebuild the module after an update. Fedora's DKMS creates its own signing key the first time and signs the module with it ([Secure Boot](#secure-boot)). SELinux in enforcing mode did not stop a snapshot in our test ([kernel 7](#kernel-7)).

On RHEL, AlmaLinux and Rocky Linux (not tested) DKMS comes from EPEL. Datto publishes dattobd packages for them; its INSTALL.md gives `sudo yum localinstall https://cpkg.datto.com/datto-rpm/repoconfig/datto-el-rpm-release-$(rpm -E %rhel)-latest.noarch.rpm`, then `sudo yum install dkms-dattobd dattobd-utils`. Datto has no packages for Fedora releases after 32.

#### Kernel 7

In November 2026 neither module builds on kernel 7.2, the kernel Fedora 44 runs after its updates, as it comes:

* elastio-snap 0.12.11 stops at `implicit declaration of function 'strncpy'` (in `main.c` and `nl_debug.c`): kernel 7 no longer has `strncpy`. It builds on Fedora 44's own release kernel, 6.19. The project's patch, [kernel7-strscpy.patch](../../packaging/linux/elastio-snap/kernel7-strscpy.patch), replaces each call with what that call needs (`strscpy`, `strscpy_pad` or `memcpy`). With it, elastio-snap builds through DKMS on Fedora 44 with kernel 7.2.14, loads at every start with SELinux enforcing, and both `elioctl` and the app's snapshot code take, mount and read snapshots. The same patched build passed the same tests on Linux Mint 21.3 with kernel 5.15. The [project's script](#elastio-snap-with-the-projects-script) applies the patch. It is offered to Axcient as [pull request 138](https://github.com/Axcient/elastio-snap/pull/138); when a released elastio-snap has the fix, the script finds it there and builds without the patch.
* dattobd 0.12.2 (its `main` branch, April 2026) fails to build on Fedora 44 with either kernel, 6.19 or 7.2.

To apply the patch by hand, fetch elastio-snap at the commit the project tested, patch it, then follow the DKMS steps of [Fedora and the RHEL family](#fedora-and-the-rhel-family) from `VER=` on:

```bash
git clone --depth 1 https://github.com/salvanza/salvanza_client.git
git clone https://github.com/Axcient/elastio-snap.git
cd elastio-snap
git checkout 971ba56a607022efc72cb3567c3cc29b2e4d10cf
patch -p1 < ../salvanza_client/packaging/linux/elastio-snap/kernel7-strscpy.patch
```

Without the patch, `dkms install` ends with "Bad return status for module build" on kernel 7, and `dkms status` shows the module as `added`, not `installed`. Remove a module that will not build with `sudo dkms remove elastio-snap/<version> --all`, so DKMS does not try again at every kernel update. A Linux machine Backup Set on a computer without a snapshot module can only read its mounted disks live: untick **Use snapshots** and tick "Run without a snapshot: I accept the backup may be inconsistent".

#### Secure Boot

Check whether Secure Boot is on:

```bash
mokutil --sb-state
```

`SecureBoot disabled`, or `EFI variables are not supported on this system` (a computer started the legacy BIOS way), means nothing more is needed. `SecureBoot enabled` means the kernel loads only modules signed with a key it trusts. **Install it for me** and the project's script enrol the key for you ([Install it from the app](#install-it-from-the-app)); the steps below are the same by hand.

DKMS signs every module it builds, as both test machines showed. The key it uses:

| Distribution | Key to enrol |
|---|---|
| Ubuntu, Mint | `/var/lib/shim-signed/mok/MOK.der` (tested on Mint 21.3) |
| Fedora, and Debian 12 or later (DKMS 3) | `/var/lib/dkms/mok.pub`, created the first time DKMS builds a module (tested on Fedora 44). `dkms install` prints it as "Public certificate (MOK)" |

With Secure Boot on, enrol that key once as a Machine Owner Key (MOK):

```bash
sudo mokutil --import /var/lib/dkms/mok.pub     # Ubuntu, Mint: /var/lib/shim-signed/mok/MOK.der
```

`mokutil` asks for a one-time password. Restart the computer: a blue MOK Manager screen appears before Linux starts. Choose **Enroll MOK**, **Continue**, **Yes**, type the password, then **Reboot**. Afterwards `mokutil --test-key <the same file>` says the key is already enrolled, and the module loads. DKMS signs later builds (after kernel updates) with the same key, so this is needed only once. On Ubuntu and Mint, installing a DKMS module on a computer with Secure Boot on usually asks for that password itself and sets up the enrolment for the next restart.

On Fedora, akmods (the way RPM Fusion builds its modules) has its own key in `/etc/pki/akmods/`; it does not sign DKMS modules, so enrol DKMS's key as above.

How a signature problem shows:

* `modprobe` says `Key was rejected by service`: Secure Boot is on and the key is not enrolled.
* `dmesg` shows `Lockdown: modprobe: unsigned module loading is restricted`: the module is not signed at all and the kernel is locked down.
* `dmesg` shows `module verification failed: signature and/or required key missing - tainting kernel`: Secure Boot is off, and the kernel loaded the module anyway. This is harmless; the test machines show it at every start.

#### Checking that it works

```bash
lsmod | grep -E 'elastio_snap|dattobd'      # loaded now
dkms status                                  # "installed" for the running kernel
ls /etc/modules-load.d/                      # elastio-snap.conf or dattobd.conf: loaded at every start
cat /proc/elastio-snap-info                  # or /proc/datto-info: the version, and any snapshots in use
```

Then close the app and start it again: it looks for the module when it starts. The machine backup screen then says "Snapshot module loaded: elastio-snap. Mounted disks are read through a snapshot." instead of the yellow "No snapshot module loaded" box. A machine backup's log has a line starting `Created elastio-snap snapshot of` (or `dattobd`) for each mounted partition.

To try a snapshot by hand with the module's own tool, on a small file system of its own (with dattobd, use `dbdctl` and `/dev/datto0`):

```bash
truncate -s 256M /tmp/snaptest.img
LOOP=$(sudo losetup --find --show /tmp/snaptest.img)
sudo mkfs.ext4 -q "$LOOP"
sudo mkdir -p /mnt/snaptest /mnt/snapview
sudo mount "$LOOP" /mnt/snaptest
echo before | sudo tee /mnt/snaptest/marker.txt
sync
sudo elioctl setup-snapshot "$LOOP" /mnt/snaptest/.cow 0
echo after | sudo tee /mnt/snaptest/marker.txt
sudo mount -o ro /dev/elastio-snap0 /mnt/snapview
cat /mnt/snaptest/marker.txt /mnt/snapview/marker.txt    # after, then before
sudo umount /mnt/snapview
sudo elioctl destroy 0
sudo umount /mnt/snaptest
sudo losetup -d "$LOOP"
rm /tmp/snaptest.img
```

#### Snapshot module problems

* **The app still says no module is loaded.** Check with `lsmod`. If the module is loaded, close the app and start it again. If not, `sudo modprobe elastio-snap` (or `dattobd`) says why.
* **`modprobe: FATAL: Module elastio-snap not found`.** It is not installed for the running kernel. `dkms status` shows the kernels it is built for. Install the running kernel's headers (`linux-headers-$(uname -r)` or `kernel-devel-$(uname -r)`), then `sudo dkms autoinstall`.
* **It worked until a kernel update.** It was probably installed with `make install`, not DKMS. Remove that copy and install it with DKMS as above.
* **`dkms install` fails.** The build log is `/var/lib/dkms/<module>/<version>/build/make.log`. The usual causes are missing kernel headers and a kernel newer than the module supports ([kernel 7](#kernel-7)).
* **`Key was rejected by service`.** See [Secure Boot](#secure-boot).
* **Removing `dattobd-dkms` fails** with "pre-removal script subprocess returned error exit status 1" (seen with 0.12.2): its script stops when the module is not loaded. Run `sudo modprobe dattobd`, then remove it again.
* **A backup stops with "refusing to back up without a snapshot".** The message says why: the backup did not run as root, no module is loaded, or both, each with what to do.
* **"elastio-snap and dattobd cannot snapshot this file system (btrfs …)".** The folder or partition is on btrfs, which neither module supports. Untick **Use VSS** for a folder set, or turn **Use snapshots** off for a machine set (`-novss` and `-liveread` on the command line).

### Where settings are kept

| | Windows | Linux |
|---|---|---|
| Settings, servers, Backup Sets, history, with the background service | `C:\ProgramData\Salvanza\` | `/var/lib/salvanza/` |
| The same without the background service | `%USERPROFILE%\.salvanza\` | `~/.salvanza/` |
| Each user's theme (and on Windows the web view's data) | `%APPDATA%\Salvanza\` | `~/.salvanza/` |
| The app's log files | `%LOCALAPPDATA%\Salvanza\Logs\` | `~/.local/state/salvanza/` (or `$XDG_STATE_HOME/salvanza`) |
| The background service's log files | `C:\ProgramData\Salvanza\` | `/var/log/salvanza/` |
| Key generated by the GUI | `keys\encryption-key.json` in the settings folder | `keys/encryption-key.json` in the settings folder |

A branded build uses folders of its own, named after its brand: `C:\ProgramData\NimbusBackup`, `~/.nimbusbackup`, `/var/lib/nimbusbackup` for Nimbus Backup, `EtitechBackup` and `~/.etitechbackup` for Etitech Backup, and so on. Before the rename to Salvanza every build used `ProxmoxBackupClient`, `/var/lib/proxmoxbackupclient` and `~/.proxmox-backup-guardian`; Salvanza moves those folders to the new names the first time it starts ([install](#2-install-and-start)).

With the background service installed, the settings are the computer's. The service keeps them, and the app shows and changes them through the service once it is connected ([section 15](#connecting-to-the-background-service)). Without the service, each account keeps its own settings in its own folder, as version 1.2 did. **Run as Service** copies yours to the service when it has none yet.

The main files are `config.json` (servers, tokens, remembered passphrases, email account), `scheduled_jobs.json` (Backup Sets), `job_history.json` (Reports) and `message_log.json` (Message Log).

**Passwords in `config.json`.** On Windows the PBS tokens and passwords, remembered key passphrases and the email password are stored encrypted with Windows' own data protection (DPAPI) for this computer: they show as `"dpapi:..."` in the file. The program that keeps the settings (the background service, or the app without one) can read them; a copy of the file on another computer cannot. An older `config.json` is converted the first time the app or the service starts. This does not keep them from an administrator of this computer, who can read anything the service can; the folder permissions below keep other accounts out. If you copy `config.json` to another computer, enter the passwords again there (or use **Export settings** with secrets, which writes them in plain text). Going back to version 1.2 or older after the conversion means entering them again too. On Linux they are stored in plain text, protected by the folder permissions.

**Who can read them.** On Windows the service's settings folder is open to SYSTEM and Administrators only. Only the service and a program started as administrator can open it: the app started normally cannot, even on an administrator's account, so it works through the service. Earlier versions also let each local administrator account read the folder without a prompt; the service removes that when it starts, and sets the permissions again each time. On Linux `/var/lib/salvanza/` belongs to root alone (folders 0700, files 0600). Earlier versions let the `sudo` (or `wheel`) group read it; the service undoes that when it starts. The service's API token, which the app needs to connect, is in the same folder. A settings folder of your own, used without the service, is private to you.

On Linux, **Restart as administrator** (`salvanza-gui-root`) runs the GUI as root. Without the background service it uses your own settings folder, and hands the settings and log folders back to you when it closes. With the service it works on the service's settings and connects without asking. A GUI started with plain `sudo` uses root's home folder, so without the service it has its own settings.

## 3. The window

The left sidebar has **Backup**, **Restore**, **Undelete** and **Roll back**, a thin divider, then **Reports**, **Message Log** and **View Full Logs**. **Preferences**, **Help** and **About** are at the bottom. The language picker (flag) is at the top right; 18 languages are available. Dates and numbers follow the app's language; with the app in English they follow the computer's regional format (Windows' Region settings; on Linux `LC_ALL`, `LC_TIME`, then `LANG`), so a British PC shows 23/10/2026, 14:05. The status bar at the bottom shows the version ("Version v..." for a release, "Development build ..." otherwise) and a **Known Limitations** link.

The menu bar:

| Menu | Item | What it does |
|---|---|---|
| File | Backup Now (Ctrl+B) | Opens the Backup page |
| File | Minimize to Tray | Hides the window to the notification area (Windows) |
| File | Exit (Ctrl+Q) | Quits the program |
| View | Reports, Message Log, Full Logs | Opens that page |
| Tools | Backup, Restore, Undelete, Roll back | Opens that page |
| Tools | Preferences… | Opens the Preferences dialog |
| Tools | Download Bare Metal Restore ISO… | Opens the latest release page in your browser |
| Tools | View Bare Metal Restore Guide | Opens the bare-metal restore manual on the Help page |
| Tools | Save Bare Metal Restore Guide… | Saves that manual as a Markdown file |
| Help | User Manual (F1) | Opens this manual on the Help page |
| Help | Command-Line Manual, Bare-Metal Restore Manual, Which Restore Do I Need? | Opens that manual on the Help page |
| Help | View Full Logs | Opens the log files in the app ([section 13](#13-message-log-and-log-files)) |
| Help | Known Limitations, About | Information pages |

**Help** shows the user manuals, built into the program so they work offline and match its version: this one, the command-line manual, the bare-metal restore manual and the restore scenarios ("Which restore?"). Pick one at the top; the contents list on the left jumps to a section, and links between the manuals stay in the app. **Find in this manual** searches the page (Enter for the next match, Shift+Enter for the previous one). **Save a copy…** saves the manual as a Markdown file, for example to read on a phone during a bare-metal restore; **Open on GitHub** shows it online at this version. The manuals are in English.

Anything that deletes, replaces or restores asks first in a dialog naming the action, for example **Delete this Backup Set?** with **Cancel** and **Delete**. For risky actions the dialog is red and Cancel is selected, so pressing Enter does nothing harmful; Escape or a click outside the dialog cancels.

## 4. Adding a PBS server

On first start, with no server configured, Preferences opens on **Account Information**. The Backup and Restore pages also show "No PBS server is configured." with **Open Preferences**. Later, open **Preferences > Account Information** and click **+ Add server**.

The server editor has four tabs.

### Server tab

| Field | Notes |
|---|---|
| Server name | A label for you, for example "Main PBS" |
| Server ID | Only when adding. Left empty, one is generated |
| PBS server URL | Full URL with port, for example `https://pbs.example.com:8007` |
| Datastore | The PBS datastore name |
| Namespace (optional) | A PBS namespace inside the datastore |
| SSL certificate fingerprint (optional) | The server's SHA-256 fingerprint. See below |
| Description (optional) | Free text |

**Certificate fingerprint.** PBS normally uses a self-signed certificate. If you leave the fingerprint empty, **Test** fetches the fingerprint the server presents and asks you to confirm it (**Trust this server's certificate?**). Compare it with the one shown on the PBS dashboard (Show Fingerprint), then click **Trust and connect**. The fingerprint is then pinned and must match on every connection (trust on first use).

**What the account may do.** **Test** also asks PBS what the account may do on the datastore (and on the namespace, if you use one). Backing up needs the **Datastore.Backup** permission; restoring other machines' backups needs **Datastore.Read** (an account with Datastore.Backup also restores the backups it made, which is all the DatastorePowerUser role gives). An account that can back up passes with no comment. An account that can only restore, with Datastore.Read but not Datastore.Backup, passes with a warning: *restore only: this account has Datastore.Read but not Datastore.Backup on /datastore/...*, because that is right for a server you only restore from, and a backup to it would be refused. An account that can do neither fails, and the message lists the permissions it has. A PBS that does not report permissions is not asked, and the test is as before.

### User / Password tab

A PBS user such as `backup@pbs`. The realm is required. The password is stored and a fresh PBS ticket is obtained for every backup.

Avoid `root@pam` or any administrator account: if this computer were compromised, that account could delete your backups. The editor shows a warning for any `@pam` user.

### API Token tab

| Field | Example |
|---|---|
| Authentication ID | `backup@pbs!laptop` |
| Secret (API Token) | the token secret PBS showed when you created it |

Create tokens in the PBS web interface under **Configuration > Access Control > API Tokens**. Give the token (or its user) the **DatastoreBackup** role on the datastore. To list and restore snapshots that another user or token made, it also needs **DatastoreReader** (or higher) on that datastore.

Fill in one of the two: User / Password or API Token. Switching to one tab clears the other.

### Encryption tab

| Control | What it does |
|---|---|
| Encryption key file (optional) | Path to a PBS key file. Empty means no encryption |
| Import... | Brings in a key kept elsewhere: a copy of the key file, or a paper key read with the camera, from a scan or photo, or typed in. It is checked and saved into the keys folder; see **Importing a key** below |
| Generate... | Creates a new key at a place you choose (default `keys\encryption-key.json` in the settings folder: the service's, when the background service is installed). The GUI makes keys without a passphrase; for a passphrase-protected key, create it with `proxmox-backup-client key create` and bring it in with Import... |
| Clear | Stops encrypting future backups to this server |

The tab shows the key's fingerprint. Every chunk is encrypted with AES-256-GCM and the manifest is signed. PBS never sees the key.

**Without the key file an encrypted snapshot cannot be restored.** Keep a copy somewhere safe, away from this computer (a password manager, a USB stick in a drawer). Under **Keep a copy of the key**:

| Button | What it does |
|---|---|
| 💾 Save a copy… | Saves a copy of the key file where you choose, for example on a USB stick. The copy is the same file: a passphrase-protected key stays protected |
| 🖨️ Print a paper key… | Shows the key's fingerprint and a QR code of the key file, and **Print** prints a page with both and the key file's text. The QR code is printed 14 cm wide so that a laptop camera can read it. To get the key back, use **Import...** (below). On Windows, when the app runs as administrator, its window cannot print, so **Print** opens the page in your web browser instead, which offers to print it: print from there and close the tab. The page is sent to the browser once, straight from the app, and is not saved as a file |

A key without a passphrase on paper is as good as the file: anyone with the page can read your encrypted backups. The paper key says so.

**Importing a key.** **Import...** asks where the key is coming from:

| Choice | What it does |
|---|---|
| Key file | A copy of the key file, for example on a USB stick (the file dialog). It is copied into the keys folder, so the stick can be put away |
| Camera | Hold the paper key's QR code up to the computer's camera, inside the square, about 30 to 50 cm away in good light. It is read as soon as it is sharp enough. Greyed out when the computer has no camera |
| Picture | A scan or photo of the paper key: JPEG, PNG, GIF, BMP, TIFF or WebP. An iPhone's HEIC photo cannot be read: save it as JPEG first |
| Type or paste | The key text printed under the QR code, or the text a phone's camera app read from the QR code. Curly quotes and stray spaces from typing are tidied up. The official `proxmox-backup-client key paperkey` layout, between its BEGIN and END lines, is read too |

Before anything is saved, the key is checked and its fingerprint shown: compare it with the one printed on the paper key. A key without a passphrase must match its own fingerprint, so a mistyped or misread character is caught ("This key has a mistake in it"). A key file written without a fingerprint cannot be checked this way; the dialog says so, and the fingerprint it shows must then be compared with the paper with care. A passphrase-protected key asks for its passphrase, which must unlock it; the passphrase is not stored, and afterwards you choose as usual whether the app asks for it or remembers it. **Save and use this key** writes it into the keys folder (`keys` in the settings folder) as `encryption-key-<first 8 characters of the fingerprint>.json`, with the same protected permissions as a generated key, and puts it in the field. An existing file is never overwritten, and a key that is already in the keys folder is not saved twice: that file is used.

A photo or scan of a key without a passphrase is enough to read the backups. The dialog says so: once the key is imported, delete the picture from your phone, email and cloud storage.

To use a key file where it is, without a copy in the keys folder (a key kept on a network share, say), type or paste its path into the field.

A key generated or imported by the GUI is written with a protected ACL (you, SYSTEM and Administrators; on Linux, readable by you only). A key whose path you type in yourself keeps its permissions; if other accounts on the computer can read it, the tab says "Other accounts on this computer can read this key file." and **🔒 Restrict access** limits it to you, SYSTEM and Administrators (on Linux, to its owner). A key on a USB stick formatted FAT or exFAT has no permissions to change and is not flagged.

**Passphrase-protected keys.** For a key protected with a passphrase (scrypt or PBKDF2), choose how the GUI unlocks it:

| Choice | Behaviour |
|---|---|
| Ask me (once per app session) | You are asked (**Passphrase required**, **Unlock**) the first time a backup or restore needs it. It is kept in memory only. Scheduled and startup backups do not prompt: they fail straight away with a clear message until you have entered it in this session. Runs handed to the background service also fail with an Ask key |
| Remember on this computer | The passphrase is stored in `config.json`, encrypted like the PBS token. Needed for unattended schedules and the background service |

**Verify** checks a typed passphrase against the key.

### Server list

Each server shows its URL, datastore and namespace, and a status dot (Online, Offline, Testing..., Not tested). The buttons are **Test**, **Edit** and **Delete**. Click **☆ Default** to make a server the default. The default (⭐ DEFAULT) is preselected for new backups and for Restore.

## 5. One-off backups

On the **Backup** page click **One-off Backup…**. A one-off backup runs once and is not saved. It always uses Legacy change detection.

| Field | Notes |
|---|---|
| Backup Name (for PBS Comment) | Optional. Shown in the Comment column in PBS |
| Backup type | **Directory (specific folder)** or **Machine (full disk)** |
| Directories to backup | Folder tree with tick boxes, including mapped network drives. Several folders are allowed. **Advanced: edit list as text** takes one path per line, including UNC paths such as `\\server\share`. For a share that needs a login, see [section 6.10](#610-folders-on-network-shares) |
| Physical disks to backup | Machine type only. Each disk shows its device, size and model, with **BOOT** and **SYSTEM** marks. **Select All** and **Deselect All** help |
| Backup as | Machine type only. See below |
| Files to exclude | Directory type only. See [exclusions](#exclusions) |
| Destination PBS server | Defaults to the default server |
| Backup ID | Defaults to the computer name. Snapshots with the same ID form one backup group in PBS |
| Use VSS (Windows Shadow Copy / Linux Snapshot) | On by default. Takes a shadow copy (Windows) or block snapshot (Linux) so open files are read consistently |
| Split this backup into multiple parts | Directory type only. For a very large first backup: it measures the folders and runs several smaller, resumable backups |

Click **Start backup**. Progress appears in the card at the top of the page ([section 8](#8-the-backup-progress-card)).

### Several folders in one backup

All folders ticked in one backup go into one snapshot, each as its own archive. In Restore they show as separate top-level folders.

### Machine backups: host or vm

**Backup as** has two choices:

* **Host backup (default).** Stored as `host/<Backup ID>`. Restore it to bare metal with the Clonezilla ISO.
* **Proxmox VE virtual machine.** Stored as `vm/<number>` so Proxmox VE lists it as a VM backup and can restore it as a VM. You type a VM ID (1 to 999999). To keep clear of your real VMs the ID is changed: **Reserved range (9000000 + ID)** (default) or **Leading zeros (000 + ID)**. The form shows the resulting "Backup ID in PBS". The VM Proxmox VE creates is bare-bones: a SATA disk, OVMF firmware for GPT disks, and no network card, TPM or drivers. The same snapshot can also be restored to bare metal with the Clonezilla ISO.

A Windows machine backup (unless the set is a forensic copy, [section 6.1](#61-source-tab)) reads free space as zeros instead of from the disk, using each NTFS volume's own allocation map, and leaves the paging, hibernation and swap files out the same way. A Linux machine backup does the same for ext2/3/4 and XFS. A half-empty disk is read in about half the time, and deleted data never reaches the backup. The image is still a complete disk. If the disk cannot read part of a partition (a bad sector), that part goes into the image as zeros and the run finishes as a partial backup that says where.

### Exclusions

One pattern per line, not case sensitive:

| Pattern | Matches |
|---|---|
| `*.tmp` | any file or folder name ending in `.tmp`, anywhere |
| `node_modules` | any file or folder with exactly that name, anywhere |
| `logs/*.tmp` | `.tmp` entries directly in `logs` under the backup folder |
| `C:\Users\Alice\Temp` | that folder and everything below it |

A pattern without a `\` or `/` matches a name anywhere in the tree. A pattern with one is anchored to the backup folder. Unticking a folder or file in the folder tree excludes just that item.

Some names are always skipped: the folders `System Volume Information`, `$RECYCLE.BIN` and `.pbs-rollback` (the Roll back safety folder) at any depth, a folder called `Recovery` only at the root of a drive (Windows' own `C:\Recovery`), and the files `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` and `DumpStack.log.tmp`. The backup log lists every skipped and excluded entry.

## 6. Backup Sets

A Backup Set is a saved backup with a name, its schedules and options. The **Backup** page lists them under **Your backup sets**. Each card shows the set's name, badges, the schedule that gives the next run and **Next run** (for scheduled sets), and **Last run** with how long ago that was ("- 2 days, 20 hours, 45 minutes ago"), which moves on each minute.

| Badge | Meaning |
|---|---|
| DAILY, REPEAT, WEEKLY, MONTHLY, YEARLY, MANUAL | The kind of schedule that gives the next run, with +1, +2... for the set's other schedules that are on. MANUAL: the set only runs on Run Now, or none of its schedules is on |
| DIRECTORY, MACHINE, MACHINE (VM) | The backup type; (VM) for a machine set stored as a Proxmox VE VM |
| FORENSIC | A machine set that reads free space too ([section 6.1](#61-source-tab)) |
| VSS ON, VSS OFF (Linux: SNAPSHOT ON, SNAPSHOT OFF) | Whether the set reads through a snapshot. Machine sets always show ON |
| VSS PARTLY, VSS N/A (amber) | The set asks for a snapshot, but some (PARTLY) or all (N/A) of its folders are network drives or shares, which cannot be snapshotted and are read live. Hover the badge for the list |
| SHARED ID (amber) | Another set writes into the same backup group on PBS ([section 6.4](#64-destination-tab)). Hover it for the other set's name |
| DISABLED (grey) | The set is out of its schedule (**More > Disable**, or the **Enabled** switch in its editor); Run Now still works |

A Metadata set with a full read set up also shows, after Last run, when its last full read finished (**Last full read:** and the date, or "not recorded yet" until one has run with this version) and **Next full read in N runs** or **Next run: full read (reads every file)**.

Each card has **Run Now**, **Edit** and a **More ▾** menu:

| Button | What it does |
|---|---|
| Run Now | Starts the set now, as its schedule would. It runs beside the backups already going, or waits its turn ([section 6.7](#67-several-backups-at-once)) |
| Edit | Opens the editor |
| More > Run now: quick (changed files only) | Metadata folder sets only. Reuses unchanged files even when the set is due its full read; the full read stays due for the next run ([section 7](#read-every-file-every-n-runs)) |
| More > Run now: full read (every file) | Metadata folder sets only. Reads every file now and restarts the full-read count |
| More > Undelete removed files... | Opens Undelete with this set picked ([section 10](#10-undelete)) |
| More > Roll back to chosen snapshot... | Opens Roll back with this set picked ([section 11](#11-roll-back)) |
| More > Clone this Backup Set | Opens the editor with a copy, named "<name> (copy)" |
| More > Disable / Enable | Disable takes the set out of its schedule but keeps it and its settings: the card shows **DISABLED** and "Next run: disabled", and Run Now still works. Enable puts it back on its schedule from the next due time, without catching up the runs it missed. The set's editor has the same choice as the **Enabled** switch beside its name |
| More > Delete this Backup Set | Asks for confirmation, then deletes the set (not its snapshots) |

Click **Add New Backup Set** to create one. Give it a **Backup set name** (the **Enabled** switch sits beside it), then fill in the five tabs: **Source**, **Exclusions**, **Schedule**, **Destination**, **Alerts**. The tabs can be clicked in any order. Under them is a row of buttons, all one width: **← Back** on the left (greyed on the first tab), then **Cancel**, **Save** and **Next →** on the right (the last tab has no Next). For a new set **Next** is the main (coloured) button, leading you through the tabs, until the last tab, where **Save** is. When you edit a set, **Save** is the main button throughout, since an edit is usually one or two changes. **Save** is also at the top of the editor, beside **✕**, which closes it. **Cancel** closes the editor and forgets what was typed. Each set needs its own name: a name another set already has (in any mix of capitals) is flagged under the name box and the set is not saved.

### 6.1 Source tab

**Backup type**, then the folders (Directory) or disks and **Backup as** (Machine), as for one-off backups ([section 5](#5-one-off-backups)).

A machine set also has **Forensic copy: read free space too**. Ticked, every sector is read, free space and the paging and hibernation files included, so deleted files can be recovered from the image later (like `salvanza-cli backup-disk -readfreespace`). Unticked (the default) they go into the image as zeros without being read ([machine backups](#machine-backups-host-or-vm)). A forensic set reads the whole disk on every run and its backups take more space. Its card shows a **FORENSIC** badge.

A machine set can leave partitions out. Under each ticked disk its partitions are listed, each with a tick box, ticked (backed up) by default. Untick one and it is not read or snapshotted, and goes into the image as zeros, which take almost no space. The partition table is kept, so a restore gives the partition back in its place and at its size, but empty and unformatted: Windows shows it as RAW and offers to format it, Linux sees an empty partition. Leaving out the partition the system runs from, or the EFI or boot partition, shows a warning: the restored disk will not start. A partition is known by its disk and where it starts. If the disk's partitions change (one moved, resized or deleted), the run backs that partition up after all and says so in its log; edit the set to choose again. On Linux a set that leaves out every mounted partition of a disk needs no snapshot for it. One-off backups always read whole disks.

A machine set also has **Folders for Undelete and Roll back**, one per line. Undelete and Roll back work on these folders, reading their files from the disk images. Left empty, they use your Desktop, Documents, Pictures, Music, Videos and Downloads. The folders must be on the disks the set backs up. Each folder's volume is found in an older disk image by its NTFS volume serial number, so a partition moved or resized since that backup is still found; without a match (another file system, or a volume formatted again) it is found by where its partition starts.

### 6.2 Exclusions tab

**Files to exclude (one per line, optional)**, Directory sets only. See [exclusions](#exclusions).

### 6.3 Schedule tab

**When does this set run?** **On a schedule**, or **Only when I click Run Now**. A set that only runs on Run Now keeps its schedules, so you can switch back later.

A set can have several schedules. It runs at the earliest time any of them gives; two due at the same minute give one run. The list shows each schedule in words ("Every month on the last day at 03:30") with a tick to switch it on or off, **Edit** and **Delete**. **Add schedule** adds one that runs every day at 02:00; **Weekly**, **Monthly** and **Yearly** start from those. A new set starts with every day at 02:00.

**Edit** opens the schedule under the list. **Repeat every** takes a number and a unit; the fields below it depend on the unit:

| Unit | Fields |
|---|---|
| minutes, hours | **All day**, or **Between** two times. Hours, all day: **At minute** past the hour. **Only on**: the days it runs (none picked means every day). The shortest repeat is 5 minutes |
| days | **At** a time. **Only on**: the days it runs |
| weeks | **On**: one or more days. **At** a time |
| months | **On** a day of the month, **the last day**, or **the** 1st, 2nd, 3rd, 4th or last weekday ("the 2nd Tuesday"). **At** a time |
| years | **On** a day and month. **At** a time |

Repeats run on the clock: every 2 hours between 08:00 and 18:00 runs at 08:00, 10:00 and so on to 18:00, however long each run takes. With a number above 1 for days, weeks, months or years, **Starting** sets the day the count runs from (every 2 weeks from this Monday). A day that a month or year does not have (the 31st, 29 February) runs on the nearest earlier day, and the editor says so. **Next 3 runs** lists when the schedule will run.

A schedule that cannot run is named in the list and the set is not saved: a repeat under 5 minutes, a time window that ends before it starts, or a weekly schedule with no day.

| Setting | Notes |
|---|---|
| Also run at machine startup | Under the list. Runs the set when the computer starts as well. With the background service, the set runs when the service starts, which is when the computer starts, whether or not anyone signs in. Without the service, it runs when the app starts |

A summary under the settings gives the set's next run and which schedule it comes from. If the computer is off or asleep at a run time, the set runs once when it is back, however many times it missed.

Schedules run while the GUI is running (in the tray is fine) or in the installed background service ([section 15](#15-background-service-tray-and-closing-the-window)).

### 6.4 Destination tab

| Setting | Notes |
|---|---|
| Destination PBS server | Which server the set backs up to |
| Backup ID | Not shown for a vm set (the VM ID decides it). A folder set gets a suggested ID made from the computer's and the set's name (below); a machine set defaults to the computer name |
| Use VSS (Windows Shadow Copy / Linux Snapshot) | As for one-off backups |
| Change detection | **Legacy: read every file**, **Data: read every file, split archive** or **Metadata: skip unchanged files (fastest, default)**, with **Read every file every N runs** for Metadata. A new folder set starts on Metadata, reading every file every 30 runs; an existing set keeps the mode it was saved with. See [section 7](#7-incremental-folder-backups-change-detection). Greyed out for machine sets |

**Give every Backup Set its own Backup ID.** All snapshots with the same type and ID form one group in PBS. Metadata mode compares with the newest snapshot in the group, and Legacy mode reuses chunks from it. If two sets share an ID, each run compares with the other set's snapshot and reads every file, and Undelete and Roll back list both sets' snapshots. A good pattern is `<computer>-<set>`, for example `fileserver-data`. The editor does this for you: a new folder set's Backup ID follows its name as you type it, the computer's name first and then the set's name without it. On `fileserver`, "Fileserver - Data" becomes `fileserver-data` and "Accounts" becomes `fileserver-accounts`, so every set of one computer sits together in PBS's list. An ID another set already uses gets `-2`, `-3` and so on, and typing your own ID stops the suggestion. A clone gets its own ID from the copy's name. Editing an existing set whose ID differs from the suggestion offers **Use <suggestion>**: a new ID starts a new group on PBS, so its first incremental run reads every file and its earlier backups stay under the old ID.

**Sets that share a group are flagged.** When another Backup Set writes to the same PBS server with the same type and Backup ID, the editor shows an amber note under the Backup ID naming that set, with **Use <suggestion>** for a folder set, and both sets' cards carry an amber **SHARED ID** badge. An empty Backup ID counts as the computer's name, and a set with no server chosen counts as on the default server. It is a warning only: saving is still allowed, for the rare case where sharing a group is what you want.

### 6.5 Alerts tab

| Setting | Notes |
|---|---|
| Send an e-mail when the backup completes | With a recipient address |
| Send an e-mail if the backup fails | With a recipient address |
| Run an application before the backup | Path or command |
| Run an application after the backup | Path or command |
| When the backup finishes, the app window: | **Stays as it is** (default), **Minimises to the tray** or **Closes (the app exits)**. Minimising needs the tray, so it does nothing on Linux, and it does nothing for a run the background service makes |
| Turn off the computer after the backup | Shuts the computer down when the set finishes. A warning shows while it is ticked. Programs still open are asked to close, not forced, so one with unsaved work can hold the shutdown up |

The actions run in this order when the set finishes: the email, the "after" application, the window choice, then the shutdown last. Closing the app or turning the computer off waits until the other backups and restores already running or waiting at that moment have finished, so it never cuts one short ([section 6.7](#67-several-backups-at-once)); a run that starts later is not waited for.

Email needs the SMTP account in **Preferences > Email** ([section 14](#14-preferences)). The tab warns if none is set up. If a successful backup's e-mail cannot be sent (the mail server is down or refuses it), "Error sending notification email, check client mail configuration." is added to the snapshot's comment on PBS, after the set's name, as the command-line tools do, so it shows in the PBS web interface.

### 6.6 What PBS sees

The set's name is written to PBS as the snapshot comment, so it shows in the PBS web interface and in the bare-metal restore menus.

### 6.7 Several backups at once

Up to three backups and restores run at the same time (**Preferences > Backup Options > Maximum Concurrent Backups**, 1 to 8). A long first backup of a large set, which can take a day or more, or a set's full read ([section 7](#read-every-file-every-n-runs)), no longer holds up the other sets: a set of documents that runs every hour keeps its hourly runs, and a machine backup still runs at its time. Whatever the number:

| Rule | Why |
|---|---|
| One machine backup at a time | Each reads whole disks. A machine backup does run beside folder backups |
| One restore at a time | |
| Nothing runs at the same time as another run of the same PBS backup group: a set never runs twice at once, and a restore or another set with the same Backup ID waits | PBS keeps one group's snapshots in order, and two backups (or a backup and a restore) of one group clash |
| Undelete and Roll back run alone | They put files back into folders other sets may be reading |

Anything that cannot start yet waits its turn, in the order it was started, and the progress card lists it with a **Stop** button ([section 8](#8-the-backup-progress-card)). A set whose run is still going, or waiting, is not started again by its schedule meanwhile; its next run is its next scheduled time after that run ends.

Each run makes its own snapshot of what it reads (VSS on Windows), so two sets reading the same drive each have their own and never see each other's. Windows makes one snapshot at a time, so a run starting at the same moment as another waits a few seconds for it. On Linux, elastio-snap and dattobd allow one snapshot of a drive at a time: a backup that needs a drive another backup has a snapshot of waits until that one has finished with it, while sets on other drives run together.

Runs at the same time share the computer's disks and network and the PBS server. Two backups reading the same hard disk slow each other down; sets on different drives gain the most. Set **Maximum Concurrent Backups** to 1 to run them one after another, as before.

Each run writes its own log file (`backup-<date>-<time>-<backup id>.log`), however many run at once ([section 13](#13-message-log-and-log-files)).

### 6.8 Speed limits

**Preferences > Backup Options > Speed limits** has three limits, in MB/s (1 MB is 1,000,000 bytes), so backups leave room on the network and disks for other work:

| Limit | What it counts |
|---|---|
| **Upload to PBS** | Everything sent to the PBS server: new chunks and the indexes |
| **Download from PBS** | Everything read from the PBS server: restores, browsing and searching a snapshot, Undelete and Roll back |
| **Disk read** | What backups read from this computer's disks: the files of a folder backup, the disks of a machine backup |

Leave a box empty (or 0) for no limit, which is the default. Each limit is shared by everything running at the same time: three backups under a 10 MB/s upload limit share 10 MB/s, they do not get 10 MB/s each. A change applies at once, also to backups and restores already running.

A backup reads and then uploads, so it goes no faster than the lower of **Disk read** and **Upload to PBS**. Data that has not changed is read but not uploaded (PBS already has its chunks), so a mostly unchanged backup is held back by **Disk read**, not by **Upload to PBS**. A Metadata backup ([section 7](#7-incremental-folder-backups-change-detection)) does not read unchanged files at all.

With the background service the limits are the service's and hold for every run it makes. Restores and browsing in the app use the same limits, but the app has its own share of them, apart from the service's.

### Upload threads

**Preferences > Backup Options > Upload threads** is how many chunks a backup sends to PBS at a time. **Auto**, the default, starts with one and allows one more for every 4 Mbit/s of upload speed it measures, up to 8. On a local network a backup soon sends 8 at a time, which is about twice as fast as one by one; on a slow link, such as a home connection to a PBS somewhere else, it sends one or two, so each chunk arrives in seconds rather than minutes and the link is not shared out so thinly that PBS goes quiet for minutes at a time. Hashing the data and checking it against the chunks PBS already has always use all the workers, whatever this is set to.

Choose a number from 1 to 8 to override Auto. A change applies at once, also to backups already running. With the background service the setting is the service's.

### Chunk timeout

A chunk upload is watched while it goes. It is given up, and the chunk sent again (up to 5 times), only when nothing has been sent for a while, or when all of it has been sent and PBS has not answered. A slow link that is still sending is not given up however slowly it goes, up to a limit of 60 minutes for one chunk, so a backup over a busy or distant connection is not cut short; a link that has stopped is noticed within a couple of minutes instead of holding the backup.

**Preferences > Backup Options > Chunk timeout** is that while: **Automatic** waits 90 seconds. A longer time suits a link that often stops for a minute or two and then carries on, such as a chain of Wi-Fi links or a mobile connection; a shorter one gives up sooner. It applies to the next chunks, also in backups already running. With the background service the setting is the service's.

### Compression

**Preferences > Backup Options > Compression** sets how hard backup data is compressed before it is sent to PBS: **Fastest**, **Default (zstd level 3)**, **Better** or **Best**. PBS stores the data as it arrives and does not compress it again, so a higher level sends less and uses less space on the datastore, for more processor time on this computer. Default suits most computers; Fastest helps a slow computer on a fast network, Better or Best a fast computer on a slow link.

The level applies to every backup this computer makes, folder and machine alike, and with the background service it is the service's. It only affects data PBS does not have yet: what is already stored stays as it was, and restores read any level, so the setting can be changed at any time. Encrypted backups are compressed too, before they are encrypted, as proxmox-backup-client does. Earlier builds used Fastest for folder backups and Default for machine backups, with no way to choose.

### 6.9 Administrator protection

With the background service installed, the Backup Sets are the computer's and the service keeps them. The app reaches them only once it has connected to the service, which needs administrator rights ([section 15](#connecting-to-the-background-service)). So adding, editing, deleting, importing and running Backup Sets, and changing servers or any other setting, already needs an administrator, on Windows and on Linux. A standard account can connect too, with an administrator's name and password.

Earlier versions had a setting for this on Windows, **Require administrator rights to add, change or delete Backup Sets** (Preferences > Advanced). It is gone, as there is nothing left for it to protect. A value an earlier version left in `HKLM\SOFTWARE\ProxmoxBackupClient\Policy` is ignored.

Without the background service each account keeps its own Backup Sets ([section 2](#where-settings-are-kept)) and can change them.

### 6.10 Folders on network shares

A folder on another computer's share (`\\server\share\folder`, or a drive letter mapped to one) can be backed up like any other. With the background service on Windows, the service reads it, and the service runs as SYSTEM, which reaches another computer as this computer's own account, not as you. In a domain where the share lets the computer in (for example **Domain Computers**), or on a share open to anyone, nothing more is needed. A share that needs a user name and password, as most do in a home or small office, refuses that account, and the backup of the folder fails with "Access is denied".

For such a share, save a login for its server:

* **When you save a Backup Set or start a backup** with a folder on a share the service cannot open, the app asks for a user name and password for that server (as `user`, `SERVER\user` or `DOMAIN\user`). It tries them on the folder before saving them, so a wrong password shows straight away.
* **Preferences > Network Shares** lists the saved logins, with **Change...** and **Remove**, and **Add login...** for another server, with a folder to try it on.

One login is kept per server and used for all its shares; Windows allows one user name per server at a time anyway. The password is stored encrypted with the service's other secrets and is never shown again. At each backup the service connects to the share with it before reading (as `net use` would), and lets go when the backup ends. If a saved login stops working (the password was changed on the server), the folders on that server fail with the reason, and the other folders of the set are still backed up.

A drive letter you map (`Z:`) exists only in your own session, and the service does not see it, so a folder on one is kept in the Backup Set as the share's own path: choose `Z:\Photos` and the set says `\\server\share\Photos`. A set saved by an earlier version with a mapped letter fails with "the background service has no drive Z:"; edit it and save it again.

Without the background service the app runs the backups itself, as you, and reads shares with your own access: no login is needed, and the Network Shares tab says so. Restores, Undelete and Roll back always run in the app, as you. On Linux, mount the share (with its own credentials, in `/etc/fstab` or by the desktop) and back up the folder it is mounted at.

## 7. Incremental folder backups (change detection)

Folder Backup Sets can use the same change-detection modes as the official `proxmox-backup-client`. This is the biggest time saver in the client.

| Mode (as shown in the editor) | Reads | Stored as |
|---|---|---|
| Legacy: read every file | every file | one `<name>.pxar.didx` per folder, as in earlier versions |
| Data: read every file, split archive | every file | a split archive: `<name>.mpxar.didx` (metadata) and `<name>.ppxar.didx` (file contents) |
| Metadata: skip unchanged files (fastest, default) | only new and changed files | the same split archive |

A new folder Backup Set starts on Metadata, with a full read every 30 runs ([below](#read-every-file-every-n-runs)). Sets saved by an earlier version keep the mode they had, Legacy included, until you change it. Machine sets and one-off backups are not affected.

### How Metadata mode works

On each run the client reads the file list of the previous snapshot in the group. A file whose size, modification time (to the nanosecond), mode and owner match is not opened at all: its data is taken from the previous snapshot's chunks. Only new and changed files are read and uploaded.

* The **first run** in Metadata mode reads every file, because there is nothing to compare with yet.
* Later runs read only what changed. Since unchanged data is never read and never sent, this also saves bandwidth to a remote PBS.
* Reuse only happens from a previous snapshot encrypted the same way, with the same key. Changing the key reads every file once.
* A file that could only be partly read on an earlier run (zero-padded after a read error) is always read again.

Real example, a 480.9 GB data share with 761,866 files on USB disks, to a PBS on the local network:

| Run | Time | Files read | New chunks |
|---|---|---|---|
| First run (no previous snapshot) | 4 h 17 min | all | 135,549 |
| Next runs, nothing or little changed, an hour or more after the last run | about 15 min | only changed files (all 761,866 unchanged files reused) | 4 to 40 |
| The same, run again within a few minutes | about 9 to 10 min | the same | the same |
| After deleting 39 GB (136 files) | about 10 min | none | 21 (about 106 MB) |

**Why the time varies.** Even an unchanged file has to be looked up on disk (size, time, permissions). Right after a run, Windows still holds those details for every file in memory, so the next run is quick. After an hour or more they have been pushed out and come off the disk again, which on a large set of small files on USB disks adds a few minutes. Nothing is wrong when a 4-hourly run takes longer than a run started straight after another.

**Deleting files still uploads a little.** The file list (names, sizes, times, permissions) is stored apart from the file contents. Where files were removed, the affected parts of the list are written again; the contents of the files that are left are reused. The space the deleted files take on PBS is freed only once the older snapshots that still hold them are pruned and garbage collection runs.

### Read every file every N runs

A file changed in a way that keeps its size and modification time is not noticed in Metadata mode. To catch such changes, set **Read every file every N runs** (0 = never, otherwise 2 to 366). Every Nth run then reads every file, like Data mode. It also drops the small amount of unused data (padding) that reuse leaves behind.

A new set starts at 30, which is about once a month for a daily set. When you save a Metadata set, a note under the editor says how often it reads every file and that **Read every file every** on the Destination tab changes it; at 0 it warns that a change which keeps a file's size and time is then never backed up. The note stays until you close it.

Only runs that finish successfully count. A stopped or failed run does not bring the next full read closer. The set's card shows **Last full read:** with the date it last finished (or "not recorded yet"), and **Next full read in N runs**, or **Next run: full read (reads every file)**.

**One scheduled full read a day.** A full read of a large set can take hours, so the computer's sets take turns: only one set a day has its scheduled full read. A set whose full read falls due on a day another set has already had one reads only changed files that time, and its full read is due again on its next run. **Run now: full read (every file)** is never held back. Together with several backups running at once ([section 6.7](#67-several-backups-at-once)), a long full read does not hold up the other sets.

As a guide: for a set that runs every 4 hours, 42 means about once a week. For a daily set, 7 means once a week and 30 about once a month.

To choose for one run, use the card's **More** menu: **Run now: quick (changed files only)** skips a full read that is due (it stays due for the next run), and **Run now: full read (every file)** reads every file now and restarts the count. The progress card then says "Full read requested: reading every file". Plain **Run Now** does what the schedule would.

### Switching an existing set to Metadata

1. Update every computer you restore from to version 2.0 or later first. Versions before split-archive support show the folders of a Data or Metadata snapshot without their files.
2. Give the set its own Backup ID if it shares one ([section 6.4](#64-destination-tab)).
3. Edit the set, **Destination** tab, choose **Metadata: skip unchanged files (fastest, default)**, check **Read every file every N runs** (it starts at 30), and save.

The first split backup of a folder shares little with its older Legacy snapshots. Expect it to use roughly one more full copy of that folder on the datastore until the old Legacy snapshots are pruned and garbage collection runs.

Machine sets ignore change detection: disks are stored as images. One-off backups always use Legacy.

**Reports** show the mode of each folder run and how many unchanged files were reused.

## 8. The backup progress card

When a backup starts, a card appears at the top of the Backup page with the set's name, a percentage and a progress bar. Every row is there from the start. A value shows **Calculating...** until it is known, and **Not available** where the run cannot provide it.

**Several backups at once** ([section 6.7](#67-several-backups-at-once)). The card shows one of them in full. **‹ 1/3 ›** beside the percentage steps through the backups running, and the card's Pause and Stop act on the one it shows. Under the buttons, **Also running** lists the others, each with its percentage (or **Paused**), **Show** to put it on the card, **⏸**/**▶** to pause or resume it and **⏹** to stop it. When the backup on the card finishes, the card moves on to the next one still running. A backup's figures (speeds, forecast) start again a second or so after it is put on the card.

The line under the bar says what the run is doing:

| Stage line | Meaning |
|---|---|
| Starting... | Connecting, taking the VSS snapshot, preparing |
| Change detection: unchanged files are reused without reading | A Metadata run, comparing with the previous snapshot |
| Scheduled full read: reading every file | This is the set's every-Nth-run full read |
| Full read requested: reading every file | You chose **Run now: full read (every file)** |
| No previous snapshot to compare with: reading every file | A Metadata run with no usable previous snapshot (first run, different key) |
| Reading every file (change detection: data) | A Data mode set |
| Reading and uploading files | A Legacy run |
| Finishing: closing the catalog and uploading the manifest... | The last step |
| Connecting to the server... / Taking a snapshot of the volumes... | A machine backup getting ready |
| Reading disk 1 of 2: CT500MX500SSD1, \\.\PhysicalDrive0 (465.8 GiB) | A machine backup reading a disk: its model, device and size |
| Finishing: closing the disk image and uploading the manifest... | The last step of a machine backup |
| Stopping... | You pressed Stop |
| ⏸ Paused for ... | You pressed Pause; the time counts up |

The rows are read down each column. The left column has the clock times, then where the backup has got to; the right column has the time spent and left, then the data on its way to the server. The rows of a folder backup:

| Row | Meaning |
|---|---|
| **Left column** | |
| Started | Clock time the run started |
| Forecast finish | Clock time it is expected to end |
| Folder | The folder being processed |
| Current file | The file being read now, updated 5 times a second |
| Files | Files done / total files. The total comes from the previous snapshot or a scan |
| Processed | Data done / total, in GB with GiB in brackets (1 GB = 1,000,000,000 bytes, 1 GiB = 1,073,741,824 bytes; the two figures always use the same step, so 1.1 GB shows as 1.0 GiB, not 1004.3 MiB) |
| **Right column** | |
| Elapsed time | Time since the start |
| Time remaining | Estimate. It uses the last 2 minutes of progress and takes the longer of the bytes-based and files-based estimates, so a run of many small files is not under-estimated. A machine backup counts only what it reads: the free space of each snapshotted volume and the partitions left out go into the image as zeros at once, so they are taken out of the total as soon as the snapshot is made, and the percentage and this estimate follow the data actually read (Data read; Position on disk still goes through the whole disk). It and Forecast finish change at most every 10 seconds, while the speed and counts update as they go. Shows **Finishing...** during the last step |
| Processing | Data processed per second over the last 10 seconds, plus files per second. Reused data counts, so this shows how fast the run is getting through the folder |
| Upload | Data actually sent to PBS per second over the last 10 seconds, compressed, as it goes over the network. Close to zero when nearly everything is reused |
| New/changed data | The size of the new chunks so far (new files, and the parts of files or disk blocks that changed), before compression. Only new chunks are sent, so this is usually far less than Processed |
| Data transferred | What the new chunks took on the network once compressed: the data actually sent to PBS so far in this run, without the few percent of connection overhead |
| Compression ratio | New/changed data divided by Data transferred, for example 6.0:1 |

A machine backup has the same right column. Its left column, under the clock times, goes from the disk down to the figure the percentage counts:

| Row | Meaning |
|---|---|
| Disk | Model, device and size, for example "CT500MX500SSD1, \\.\PhysicalDrive0 (465.8 GiB)" |
| Partition | For example "C: (NTFS)", "Partition 2" or "Unpartitioned space"; on Linux "/dev/sda2 (/, ext4)" |
| Blocks | 4 MiB blocks of the current disk, done / total |
| Position on disk | How far through the current disk the backup is, free space included |
| Data read | The data read so far / the data there is to read on all the disks: the disks without their free space and the partitions left out. This is what the percentage counts, so a mostly empty disk shows 99% here while Position on disk is still far from the end |

The box underneath shows the number of **New** and **Reused** chunks. A machine backup adds how many of the reused chunks are blank space, how much goes into the image without being read (free space or partitions left out), and in red how much of the disk could not be read (zeros in the image). Failed chunks show in red.

**Why a repeat run still sends data.** A machine backup stores a disk in 4 MiB blocks. A block in which anything changed, even a few bytes of a log or database file, is a new chunk and is counted whole as New/changed data. Compression then shrinks it before it is sent, which is why Data transferred is often several times smaller. On a metered or slow link, Data transferred and Upload are the figures to watch.

**When the connection is lost.** If the connection to PBS drops during a machine backup (a mobile link losing signal, PBS restarting), PBS ends that backup, so the app starts it again with a new snapshot of the disks: after 2 minutes, then 5, 10 and 15 if it keeps failing, five attempts in all. The line under the bar says "Connection to the server lost: starting again at ..." while it waits, and **Stop** ends the wait. The new attempt sends again the chunks the lost one had sent, as PBS keeps only what a finished backup refers to, and the figures start again from zero. A folder backup does the same once, after waiting 25 minutes.

Real example, a small Linux PC at an unoccupied remote site, recording two cameras with live detection and running home automation, backing up over a 4G mobile connection (about 4.8 Mbit/s upload) and a VPN to a PBS at home:

| Run | Time | New chunks | New/changed data | Data transferred |
|---|---|---|---|---|
| Whole 128 GB disk image (an earlier image of it already on PBS), then the home folder (folder backup, Metadata mode, 346 unchanged files reused) | 10 min 38 s, then 14 s | 213 of 30,527 (25,418 of them blank), then 12 | 861 MiB | about 172 MiB |
| The same disk again, two minutes later | 5 min 32 s | 76 | 300 MiB | about 46 MiB |

The PC kept recording without dropping frames throughout. The network figures above were measured at the VPN server, before the app counted them, so they include the connection overhead. On a later run the app showed 508.3 MiB of new/changed data and 84.6 MiB transferred (6.0:1); the VPN server counted 91.1 MiB from the PC over the same half hour, other traffic included.

**Pause** holds the run where it is, part way through a file if need be, and the button becomes **Resume**. Nothing is read or uploaded while it is paused, and the connection to PBS stays open, so Resume carries on from the same place. Stopping a backup and starting it again would read everything from the beginning. Use it when the PBS server or the network is busy with something else, or the computer is needed for a while. While a backup is paused:

- only this backup waits: other backups and restores carry on, and it keeps its place among the backups running at once;
- the VSS snapshot stays in place, so its shadow storage grows as files on that drive change;
- if the connection to PBS drops (the server restarts, the network goes down) PBS ends the backup as it would while running, and the app starts it again as described under **When the connection is lost** above;
- Forecast finish and Time remaining show **Paused**, and pick up again from fresh measurements after Resume.

Pause works for folder and machine backups, whether the app or the background service runs them. With **Run as Service** on ([section 15](#15-background-service-tray-and-closing-the-window)), a Backup Set the service starts on its schedule shows the same card in an open app window, with its set's name, within a few seconds, and Pause, Resume and Stop act on the service's run. A window opened during the run picks it up, paused or not. On Windows the tray icon's menu has **Pause backup** too (**Resume backup** while paused), so a run can be paused without opening the window ([section 15](#15-background-service-tray-and-closing-the-window)). The card of a run the service started on its own fills in as for a run of the app's own: the stage line, the percentage, and the Files, Folder, Upload and data rows. (A background service from version 1.2 or earlier sends only its percentage, and the other rows then read **Not available**.)

**Stop** ends the run at once, part way through a file if need be, and also works while paused. No snapshot is kept for the stopped run, so the next backup compares with the last complete one. Reports record it as cancelled (⏹️), not failed.

Anything waiting for its turn is listed under the card, each backup with a **Stop** button that ends its wait (it is recorded as cancelled).

## 9. Restore

Open the **Restore** page. It is a wizard of five steps, shown as chevrons across the top: **1 Source**, **2 Snapshot**, **3 Files**, **4 Destination** and **5 Options and restore**. Each step asks for one thing:

* **Next** (for example **Next: list snapshots →**) goes on, and **← Back** goes back. Next is greyed, with what is missing as its tooltip, until the step is filled in.
* From the second step on, a summary bar names what you have chosen so far (server, Backup ID, snapshot, files), with a **Change** link back to that step.
* A step not reached yet is greyed, with what it needs as its tooltip. Any step already open, before or after the one shown, can be clicked. A step gets a tick once the step after it is open.
* Changing an earlier choice (another server, Backup ID or snapshot) clears what depended on it, and the wizard goes back to the step that is still valid.
* The restore card ([section 9.5](#95-the-restore-card)) and the list of restores waiting their turn stay above the steps on every step.
* Clicking **Restore** in the side menu starts the wizard afresh, unless it is busy listing or searching or a restore is running.

The box on the first step lists what restore supports: files and folders (full tree), selective restore, modification times, creation times (Windows), several PBS servers, NTFS ACLs (Windows) or POSIX ACLs (Linux), and alternate data streams and NTFS extended attributes (Windows; folder backups made by version 1.1 or earlier, or by a test build from before 14 October 2026, lack them).

A folder backup made on Windows can be restored on Linux too (to another folder). Where the app gives an example path, it gives a Linux one on Linux (`/home/name/Restore`).

### 9.1 Find the snapshot

**Step 1, Source** ("Where is the backup?"):

1. **Source PBS server**: pick the server.
2. **Backup ID to restore (optional)**: type an ID, or use the buttons beside it:
   * **▾** lists the Backup IDs your Backup Sets use (with the set names), plus this computer's name for one-off backups.
   * **✕** clears the field. An empty field lists every backup in the datastore, which is what you want when restoring onto a replacement computer.
3. Click **Next: list snapshots**. A moving bar shows while PBS answers. If it takes more than 15 seconds the GUI says PBS is slow to respond (a verify, garbage collection or backup may be running on the server).

**Step 2, Snapshot** ("Choose a snapshot") lists the snapshots, newest first, with date, Backup ID, type and size, in **Tile** or **List** view. 🔒 marks a protected snapshot, and a machine backup is marked "disk image". Click a snapshot (or its **Select**) to choose it; the wizard goes on to its files.

The list shows a page at a time. Above it, and again below it when there is more than one page, the arrows **«** **‹** **›** **»** go to the first, previous, next and last page either side of "Showing 11-20 of 120". **Show** sets how many a page holds: 5, 10 (the default), 25, 50 or All; the choice is kept, and Roll back's list uses the same one. **At or before:** takes a date and time and lists only the snapshots at or before it; **✕** clears it and goes back to the page with the chosen snapshot.

### 9.2 Search for a file

On step 1, click **🔍 Find a file instead** to open the search panel. It searches every snapshot whose Backup ID starts with the ID in the box, over a date range.

| Setting | Notes |
|---|---|
| Search term | For example `invoice`, `*.docx`, `report-2024` |
| Type | **File name**, **Regular expression** or **Full path** |
| From / To | Date range of snapshots to search |
| Also assemble snapshots not yet browsed | Slower: downloads the file lists of snapshots not already cached |

Click **Search**. Each result shows the file's name, its original path (for example `D:\Data\Documents\Invoices\INV-0412.pdf`), the Backup ID and the snapshot's date. Results stop at 5,000; refine the search if you reach it. **Prepare restore** on a result chooses that snapshot and ticks that file, and the wizard goes straight to step 4, Destination.

### 9.3 Choose what to restore

**Step 3, Files** ("Choose what to restore") shows the snapshot's **Original source**, **Machine** and **Backed up at**, and the snapshot's contents as a tree. Folders load one level at a time; a folder with many entries shows the first 1,000 with **Show N more**.

* Tick the files and folders to restore. **Select all** and **Select none** help. Nothing ticked means nothing is restored.
* The line under the tree shows how many entries are ticked and their total size.
* **Reload** downloads the tree again from PBS instead of the local cache.

Selective restore reads only the chunks that hold the ticked files, so restoring a few folders from a large snapshot is quick. Click **Next: destination**.

### 9.4 Choose where and how

**Step 4, Destination** ("Where should the files go?") has the first three settings below; **Next: options** opens **step 5, Options and restore**, with the rest, and **Ready to restore**, a list of everything chosen.

| Setting | Notes |
|---|---|
| Restore to original path | Writes back to where the files came from. Overwrites existing files. Not available when the snapshot has no record of its original path, was made on another operating system, or is a disk image. If the snapshot came from another computer you must tick "Force cross-host" |
| Restore to alternate path | Writes to the **Destination folder** you choose (**Browse**) |
| Keep original directory structure | Alternate path only. On: `dest\Users\alice\doc.txt`. Off: `dest\doc.txt` (good for a single file) |
| Overwrite existing files | Always on for original path |
| Restore modification times | Sets file and folder modification times as they were in the backup. Untick it to leave restored files with the time of the restore. Creation times (Windows) are put back either way. Undelete and Roll back always put the times back |
| Verify after restore | Reads every restored file back and compares it with the snapshot. Mismatches are reported; nothing is deleted |
| Restore permissions/ACLs | Windows: owner, group, permissions (DACL) and Hidden/System/Archive/ReadOnly attributes. Linux: POSIX ACLs and extended attributes. Run as administrator (or root) for this |
| Restore alternate data streams and extended attributes | Windows, on by default. Puts back the data attached to files and folders, such as the "downloaded from the internet" mark (Zone.Identifier), and NTFS extended attributes (WSL keeps Linux owners and permissions in them). Streams over 1 MB are recorded by name only and are not restored (the restore log says how many). Folder backups made by version 1.1 or earlier, or by a test build from before 14 October 2026, did not record them; disk images of any version give them back. Not shown on Linux |

On Windows the **creation time** (Explorer's "Date created") of every file and folder is put back too, whether or not permissions are restored, for backups made by version 1.2 or later; version 1.1, and test builds from before 11 October 2026, did not record it.

Click **▶️ Restore** (with **Stop** beside it). The result shows under the options until another restore starts. Once a restore has finished, its button reads **✓ Completed** and is greyed, so the same restore is not started twice by mistake; change any choice (files, destination, an option) and it reads **Restore** again.

### 9.5 The restore card

The restore card works like the backup card and stays at the top of the page, whichever step is open. Its stage line shows **Connecting and preparing restore...**, **Reading snapshot index and locating files...**, **Fetching chunks and writing files...**, then, after the data, **Setting folder timestamps...**, **Restoring ACLs and attributes...** (or **Restoring creation times...** when permissions are not being restored) and **Verifying restored files...** as they apply. The rows are Started, Forecast finish, Elapsed time, Time remaining, Speed (shown as the average once the transfer is done), Downloaded (data done / total) and Current file. Under them a box of two lines gives the restore's own detail ("Restoring Photos.ppxar.didx: 12.5 GB ... written"); it keeps its height, so the card does not jump as the text changes, and longer text shows whole on hover. **Stop** ends the restore.

A restore of several backed-up folders is followed as one: the bar, Downloaded, Time remaining and Forecast finish cover the whole restore, each folder weighed by its size (a partly ticked folder by the size of what is ticked). Two more rows show the folder being restored: **Current folder:** with its full original path, cut in the middle with "..." when long, and which it is ("(2 of 3)"), and **This folder:** with how much of it is done.

The finished restore is recorded in Reports with its snapshot, destination, paths, file count, size, verification result and duration.

### 9.6 Files from a machine backup

A machine backup (a `host` or `vm` snapshot of whole disks) opens in Restore like a folder backup. Each volume in the image is a top-level folder, for example "Partition 3 - Windows (NTFS, 127.3 GB)". A disk with a file system but no partition table shows as "Whole disk", and with several disks the folders start with "Disk 2" and so on. Nothing is mounted and the image is not downloaded: the partition table and the volume's file table are read straight from the backup, then only the chunks holding the chosen files.

* **File systems**: NTFS, FAT12/16/32 (EFI partitions, memory cards), exFAT, ext2/3/4 and XFS. Swap, LVM and unknown partitions are left out. A BitLocker partition cannot be browsed, and the restore log says why.
* **Where to**: files always go to another folder (Restore to alternate path). They are never written over the live disk.
* **What comes back**: file contents, modification and creation times, and on Windows the alternate data streams (such as Zone.Identifier) and extended attributes (such as the ones WSL keeps for Linux owners and permissions) of NTFS files and folders, unless **Restore alternate data streams and extended attributes** is unticked. With **Restore permissions/ACLs** ticked, also the read-only, hidden, system and archive attributes and, on Windows, the NTFS permissions. Files from ext2/3/4 and XFS volumes keep their symbolic links and, restored on Linux, their permissions (and owners when restoring as root).
* **Speed**: a 127 GB laptop C: with 430,000 files lists in 4 to 15 seconds. Each volume's file list is kept on this computer once read, so opening the same snapshot again, even after a restart, is quick. `System Volume Information` is left out.

The same reading is what Undelete and Roll back use for machine sets.

## 10. Undelete

**Undelete** (side menu, or **More > Undelete removed files...** on a Backup Set's card) finds files that are in a Backup Set's backups but no longer on disk, and puts them back. Like Restore it is a wizard ([section 9](#9-restore)), with three steps: **1 Set**, **2 Deleted files** and **3 Destination and undelete**, a summary bar with **Change**, and **← Back** and **Next** buttons. Choosing Undelete in the side menu, or on a card, starts it afresh unless it is busy.

**Step 1, Set** ("Which files went missing?"):

1. Pick the **Backup Set**. A machine set is listed with the folders it covers ([section 6.1](#61-source-tab)); their file lists are read from the disk images (NTFS on Windows; ext4 or XFS on Linux, where folders on LVM or RAID are left out with the reason).
2. Choose how far to **Look back**:

   | Choice | What is checked |
   |---|---|
   | Latest backup | Files in the newest backup that are not on disk now |
   | Last 7 days / Last 30 days | Every backup of that period. Each missing file is offered from the **newest** backup that still has it |

3. Click **Find deleted files →**. The GUI reads each backup's file list (its catalog, a few MB even for hundreds of thousands of files, kept on this computer afterwards so the next search is quicker) and lists the set's folders on disk. No file is opened or read. The set's exclusions apply, so excluded files never show up.

   The bar shows where it is: **Reading the file list of snapshot 2 of 6**, then **Reading the folder F:\Data: 312,400 of about 868,592 files and folders** (the total is what the newest backup had there, so it is an estimate), with the time taken. As a guide, for a set of 760,000 files on USB disks: a backup's file list takes about 10 seconds the first time and a second or two after that, and listing the folders takes from about 30 seconds (just after a backup) to 3 or 4 minutes (disk not read for a while). A 7-day search typically takes 1 to 5 minutes.

When the search ends the wizard moves on to **step 2, Deleted files**: a folder tree of the missing files with a tick box on every file and folder. Each file shows its size and when it went: **deleted since** the newest backup, or **deleted between** two backups. A file whose name, size and time match a file somewhere else in the folder is marked **probably moved to ...** and is not ticked, since it most likely still exists under another name or folder. Files under a folder that cannot be read now (permissions) are not listed as missing; a note says how many.

**A set folder the logged in user cannot open.** Undelete runs in the app, as you, while the backups may run in the background service with a login saved for a network share ([section 6.10](#610-folders-on-network-shares)). A set folder you cannot open yourself is left out, and the others are still checked, with a note: "Couldn't open `\\server\share\Photos` as the logged in user, so nothing there was checked. If it is a network share, open it once in File Explorer, then try again." (on Linux: "in the file manager"). Only when no folder of the set opens is it an error, with the same wording. A folder that does not exist at all is still an error.

Use the filter box to narrow the list, and **Select all shown** and **Clear shown** for what is visible. The line under the tree counts the files ticked and their size. Click **Next: destination →** for **step 3, Destination and undelete** ("Where should they go?"), and choose **Restore to**:

| Choice | What happens |
|---|---|
| Where they were | Each file goes back to its original place. A file that exists there again is left alone and reported as skipped |
| Another folder | The files go under the folder you choose (**Browse...**), keeping their folder structure |

Tick **Verify after restore** to check every restored file against the backup. **Ready to undelete** lists the set, the files and the destination. Click **Undelete N files (size)**. The restore card shows the progress (several backups are restored one after another), Stop works as usual, and Reports records the run as **Undelete**. When it has finished the result shows above the buttons, with **Find deleted files** to search again, and the button reads **✓ Completed**, greyed, until you change the files, the destination or **Verify after restore**.

Undelete also runs from the command line for any Backup Set ([below](#undelete-and-roll-back-from-the-command-line)).

## 11. Roll back

**Roll back** (side menu, or **More > Roll back to chosen snapshot...** on a Backup Set's card) puts a Backup Set's folders back as they were at a chosen backup. You see what would change first, and everything it replaces or removes is kept, so the roll back can be undone. For a machine set it works on the set's **Folders for Undelete and Roll back**, reading them from the disk images.

Like Restore it is a wizard ([section 9](#9-restore)), with four steps: **1 Set**, **2 Backup**, **3 Changes** and **4 Roll back**, a summary bar with **Change** (or **Change backup**), and **← Back** and **Next** buttons. Choosing Roll back in the side menu, or on a card, starts it afresh unless it is busy listing, comparing or undoing.

1. **Step 1, Set** ("Which folders?"): pick the **Backup Set**. Its earlier roll backs are listed here, with **Undo** (see **Earlier roll backs of this set** below). Click **Next: choose a backup →**.
2. **Step 2, Backup** ("Roll back to which backup?"): the set's backups, newest first, with their size; 🔒 marks a protected snapshot. The list is paged as Restore's snapshot list is ([section 9.1](#91-find-the-snapshot)), with the same **Show** choice. Pick a backup, or enter a date and time in **The backup at or before:** to pick the newest backup at or before it and list only those.
3. Click **Show what would change →**. Nothing is written. **Step 3, Changes** compares the backup with the folders now:

   | Box | Meaning |
   |---|---|
   | Missing now | In the backup, gone from disk: restored |
   | Changed since | Different size or modification time: replaced by the backup's version |
   | Added since | On disk but not in the backup: kept, unless you choose an exact roll back |
   | Unchanged | Left as they are |

   The files of each group are listed underneath (the first 2,000 of each); click a group's name to open its list.

   A set folder the logged in user cannot open (a network share only the service's saved login reaches, as for Undelete above) is left alone: nothing in it is compared, moved or restored, and the preview says "Couldn't open ... as the logged in user, so it is left alone.", with the same hint. Only when no folder of the set opens is it an error. If such a folder opens by the time you start, the roll back stops without changing anything, since that folder was never compared: show what would change again.

4. Click **Next: roll back options →**. On **step 4, Roll back**, choose **What to do**:

   | Choice | Effect |
   |---|---|
   | Restore missing files and replace changed ones; keep files added since | The usual roll back (default) |
   | Restore missing files only | Nothing on disk is replaced; changed files keep their current version |
   | Exact roll back: also remove files added since | The folders end up exactly as in the backup. You confirm by typing the number of files that will be removed |

5. **Back up the set first** runs the Backup Set once before anything changes, so today's state is a backup too. It is on by default for a folder set and off for a machine set, whose backup is a whole-disk image and takes about as long as its usual backup. While that backup runs, the card reads "Backing up the set before the roll back: 37%". **Verify after restore** checks the restored files. Then click **Roll back to ...** (the backup's date). When it has finished, the wizard goes back to step 1, with the result above the set's roll backs and the new one's **Undo**.

**The safety net.** Before a file is replaced or removed it is moved, not deleted, into a hidden `.pbs-rollback` folder on the same drive. Moving within a drive is instant, whatever the file size. For folders inside a user's own profile or home (`C:\Users\<name>\...`, `/home/<name>/...`) it is in that profile (`C:\Users\<name>\.pbs-rollback\<set name>\<date-time>\`), so a user rolling back their own files needs no administrator rights; for anything else it is at the root of the drive (`F:\.pbs-rollback\...`). Only SYSTEM (root), the administrators and the account that ran the roll back can open it: it holds copies of the set's files. The folder also holds `rollback.json`, the record of what was moved, restored and removed; Undo and Delete kept versions refuse a record that names anything outside the set's folders or its own safety folder. `.pbs-rollback` is never backed up. Files that are open in another program cannot be moved; they are left as they are and listed in the result, with the reason.

While a roll back runs, the set's own scheduled backups wait, and Run Now on that set is refused. Reports records the run as **Roll back**.

**Earlier roll backs of this set** are listed on step 1, under the set, with what each restored, replaced and removed and how much the kept versions take up:

| Button | What it does |
|---|---|
| Undo | Deletes the files the roll back restored, moves every kept version back, makes removed folders again and removes folders the roll back created if they are empty. The safety folder is removed when everything went back |
| Delete kept versions | Deletes that roll back's safety folder to free the space. It can no longer be undone |

### Undelete and Roll back from the command line

The service program, `salvanza-svc.exe` on Windows (in `C:\Program Files\Salvanza`), also runs Undelete and Roll back from the command line, for scripts and servers without a desktop, for a Backup Set defined in the GUI: the same servers, folders, exclusions and safety folder. On Linux the app takes the same options: run `salvanza-gui` (the package's launcher in `/usr/bin`) or `/usr/lib/salvanza/salvanza-gui`, which then runs on the command line without a window; the package's `/usr/libexec/salvanza/salvanza-svc` takes them too. On Windows the app itself has no console, so use the service program. Run it as administrator (Windows) or root (Linux), as the backups run. (These options belong to the GUI's programs, not to `salvanza-cli`, which only backs up: [CLI.md](CLI.md).)

The service program works on the background service's Backup Sets, in `C:\ProgramData\Salvanza` or `/var/lib/salvanza`, which only administrators (Windows) or root (Linux) can open. With the background service installed, use it for the sets the service runs. The Linux app works on the sets in `~/.salvanza`, as it does without the service.

```
salvanza-svc.exe -list-sets
salvanza-svc.exe -undelete "Documents" [-days 7] [-to D:\Recovered] [-dry-run] [-no-verify]
salvanza-svc.exe -rollback "Documents" [-at "2026-10-10 14:30"] [-policy replace|missing|exact]
                 [-backup-first] [-dry-run] [-yes] [-no-verify]
salvanza-svc.exe -list-rollbacks "Documents"
salvanza-svc.exe -undo-rollback "Documents" -created 2026-11-08T14:30:00Z
salvanza-svc.exe -delete-rollback-copies "Documents" -created 2026-11-08T14:30:00Z
```

| Option | Meaning |
|---|---|
| `-list-sets` | Lists the Backup Sets: name, id and folders (or disks) |
| `-undelete <set>` | Lists the set's files that are in its newest backup but not on disk, then restores them where they were (never over a file that has come back) or under `-to`. Files that look moved (same name, size and time elsewhere) are listed, not restored |
| `-days N` | Undelete: also looks in the backups of the last N days; each file comes from the newest backup that has it |
| `-to <folder>` | Undelete: restore under this folder instead of where the files were |
| `-rollback <set>` | Puts the set's folders back as the newest backup (or the one at or before `-at`) has them. Shows the counts first; asks for `yes` unless `-yes`. Everything replaced or removed is moved to the safety folder first, so Undo in the GUI can put it back |
| `-at "YYYY-MM-DD HH:MM"` | Roll back: use the newest backup at or before this local time |
| `-policy` | `replace` (default: restore missing and changed files, keep files added since), `missing` (missing files only) or `exact` (also remove files added since) |
| `-list-rollbacks <set>` | Lists the set's earlier roll backs that left a safety folder, newest first: one line each with the time, state, policy and counts |
| `-undo-rollback <set>` | Undoes one roll back, the one made at `-created` (the time `-list-rollbacks` printed): the files it restored are deleted and the files it moved aside go back |
| `-delete-rollback-copies <set>` | Deletes the safety copies of the roll back made at `-created`; it can no longer be undone |
| `-backup-first` | Roll back: back the set up before changing anything |
| `-yes` | Roll back: do not ask for `yes` |
| `-dry-run` | Only show what would be done |
| `-no-verify` | Do not check the restored files against the backup |

A set is named by its name (any case) or its id. Undelete does not ask before it restores (it never replaces a file), so use `-dry-run` to look first. The exit code is 0 when it worked, 1 on any failure (or when a roll back was not confirmed), 2 for bad options. Machine sets work too: their folders are read from the disk images.

There is no command line option for a full restore; use [Restore](#9-restore), or the official client ([section 16](#16-compatibility-with-the-official-proxmox-client)).


## 12. Reports

**Reports** lists every backup and restore run, newest first, with ✅ success, ❌ failed, ⏹️ cancelled or ⏳ running. The list also shows 🔒 Encrypted or Unencrypted, and for Metadata runs how many files were reused.

Click a run for its details:

| Detail | Notes |
|---|---|
| Status, Date / Time | |
| Mode | Automatic (scheduled), Automatic (at startup), Manual (Run Now) or Manual (one-off) |
| Type | Folders, Full machine, Restore, Undelete or Roll back |
| Encryption | Encrypted or Unencrypted |
| Change detection | Legacy, Data (split archive) or Metadata (unchanged files skipped), with "N unchanged files reused without reading, size" |
| Duration, Backup size, Chunks, New/changed data, Data transferred, Compression ratio | A finished backup's figures. Chunks gives the total, New, Reused and, for a machine backup, Blank (reused chunks that are blank space). New/changed data is the size of the new chunks; Data transferred is what they took on the network once compressed (backups made before this version show New/changed data only). Warnings gives the number of files the backup could not read; Left out gives the links, sockets, pipes and system files it leaves out on purpose, which is not a problem. The backup log names each one |
| Message | The result message, for a failed run, a run with errors or a machine backup with unreadable sectors (where they are) |
| Backup ID, VSS, Folders | |
| Snapshot, Destination, Paths, Files, Size, Verification, Duration | Restores, Undeletes and Roll backs |

A failed backup has a **Rerun** button that loads its folders and Backup ID into the Backup page. **📄 View log** opens the run's own log file in **View Full Logs** ([section 13](#13-message-log-and-log-files)); for a restore, or a backup whose own log is no longer there, it opens the backup log at the run's time.

## 13. Message Log and log files

**Message Log** is a timestamped list of backup and restore messages (✅ finished, ℹ️ information, ⚠️ warning, ❌ error, ⏹️ stopped by you) with their source, capped at 1,000 entries. A finished backup shows its figures underneath, as in Reports: duration, backup size, chunks, new/changed data, data transferred and the compression ratio. It has a ✅ when it read everything it set out to, also when it left out links, sockets, pipes or system files on purpose (listed as "items left out by design"), and a ⚠️ when it could not read some files.

Each line has a **📄** button that opens the log at that message: a backup's own run log, or for anything else the app's log at the message's time.

**View Full Logs** (in the side menu under Message Log, and on the **Help** and **View** menus) shows the log files themselves, in the app. The list on the left has every log file, newest first: a backup run's own log ("Backup run: fileserver-data", with when it started), the backup and restore log, and the app and schedules log, marked "service" when the background service wrote it, with its size. The one picked shows on the right, its last 2,000 lines to begin with, or, opened from Reports or the Message Log, the lines around that time with the line itself highlighted:

* **Show only lines containing…** then **Show** (or Enter) keeps only the lines with that text, in any case, for example `skipped`, `ERROR` or a file name.
* **Hide chunk lines** (ticked to begin with) leaves out the "New chunk" and "Reuse chunk" lines, one per 4 MiB of data, which make up most of a run's log.
* **Show more lines** shows four times as many lines, up to 200,000. The line above the box says how many lines are shown of how many there are.
* Errors show in red and warnings and skipped files in amber. Compressed (`.gz`) logs are read as they are.
* **Refresh** reads the list and the log again; **Open log folder** opens the folder.

On Windows the app's log files are in `%LOCALAPPDATA%\Salvanza\Logs\`, and the background service's in `C:\ProgramData\Salvanza\`, which only administrators can open (Linux: [section 2](#where-settings-are-kept)). **View Full Logs** lists both: the service's come through the service, once the app is connected to it. **Preferences > Message Log > View Logs** opens the app's log folder.

| File | Contents |
|---|---|
| `backup-YYYYMMDD-HHMMSS-<backup id>.log.gz` | One per backup run, compressed when the run ends. Open with 7-Zip or `zcat` |
| `backup-gui.log` | General backup and restore log of the GUI |
| `service-gui.log` | Scheduler and general operations of the GUI |
| `backup-service.log`, `service-service.log` | The same for the background service |

Logs rotate at 10 MB, keeping 5 old files.

Every run log ends with a `[RESULT]` line, for example:

```
[RESULT] outcome=verified_success dirs_ok=1/1 new=9 reused=158838 failed=0 read_errors=0 excluded=0 bytes=480858587134 new_data=37748736 transferred=9215932 duration=10m24.4s
```

A Metadata run also logs how many unchanged files were reused, how many chunks were reused and how much padding that added. A machine backup logs one progress line per 1%.

## 14. Preferences

Open Preferences from the side menu or **Tools > Preferences…**. The dialog has **OK**, **Cancel** and **Help** at the bottom; each setting is saved as you change it (or with the tab's own **Save** button). **Help** opens this section of the manual.

| Tab | Contents |
|---|---|
| Account Information | PBS servers ([section 4](#4-adding-a-pbs-server)) |
| Backup Options | **Parallel restore extraction (experimental)**: writes several files at once during a restore. Not always faster; on our test machine it was slightly slower. **Multi-threaded backup reading (experimental)**: reads files ahead in the background during a backup, with a **Worker count:** (1 to 32, default 8). Helps when the disk is the bottleneck. Turn either off if anything behaves unexpectedly. **Maximum Concurrent Backups:** how many backups and restores may run at the same time, 1 to 8, default 3 ([section 6.7](#67-several-backups-at-once)). 1 runs them one after another, as before. A change applies to the next backup to start. **Speed limits**: **Upload to PBS**, **Download from PBS** and **Disk read**, in MB/s, empty for no limit, each shared by everything running at the same time ([section 6.8](#68-speed-limits)). **Upload threads:** how many chunks go to PBS at a time ([Upload threads](#upload-threads)). **Chunk timeout:** how long a chunk upload may stop moving before it is sent again ([Chunk timeout](#chunk-timeout)). **Compression:** Fastest, Default (zstd level 3), Better or Best, for every backup ([Compression](#compression)) |
| Network Shares | The logins the background service uses for folders on network shares, one per server, with **Change...**, **Remove** and **Add login...** ([section 6.10](#610-folders-on-network-shares)) |
| Theme | Amber, Classic Blue, Green, Red, Dark, or Custom… (your own Accent color, Accent (hover) and the two gradient ends, as 6-digit hex values, with a preview). Each user has their own theme: until they choose one it is Green, or after an upgrade the theme the computer had. A branded build (one whose program file carries a brand name the app knows) uses the brand's own colours and has no Theme tab |
| Email | The SMTP account all Backup Sets use: SMTP server, Port, Username, Password, From address, and "Allow an unencrypted connection (not recommended)". For a relay that takes mail without a login, leave Username and Password empty and fill in From address. The From address may include a name, for example `Backups <backup@example.com>`. **Save**, then **Send test email** to an address to check it ("Sending..." with a wait bar while it goes). Each set picks its own recipient on its Alerts tab |
| Export / Import Settings | **Export Settings...** saves your servers, the default server, theme, email account and Backup Sets to a `.json` file. Tick "Include passwords/tokens in the export" to include the PBS tokens and passwords, remembered key passphrases, the SMTP password and the network share passwords, in plain text, so keep that file safe. The network share logins' servers and user names are always included. Without it, none of them is written. Encryption key files are not included: copy them yourself (**Save a copy…** on the Encryption tab). **Import Settings...** loads such a file, for example from another computer. Servers and Backup Sets with the same ID are replaced, others are added, and nothing else is removed. With the background service, export and import go through the service, as every other change does. The same file sets up many computers at once when the Windows installer is given it (`CONFIGFILE`, see [installer/DEPLOYMENT.md](../../installer/DEPLOYMENT.md)): there, a set whose backup ID is the exporting computer's name backs up under each computer's own name |
| Message Log | **View Logs** opens the log folder |
| Advanced | **Run as Service** ([section 15](#15-background-service-tray-and-closing-the-window)) |

## 15. Background service, tray and closing the window

**Closing the window on Windows** (the X) asks whether to **Minimize to Tray** or **Exit**. Minimised, the GUI keeps running in the notification area and its schedules still run. **Exit** (also **File > Exit**) quits completely, and no schedules run from the GUI until you open it again.

The **tray icon** (Windows) menu has Show window, Pause backup, Backup status and Quit. **Pause backup** can be clicked while a backup runs, whether the app or the background service runs it, and pauses it as the progress card's Pause does ([section 8](#8-the-backup-progress-card)); with several backups running it pauses them all. It then reads **Resume backup** (which resumes them all), and the icon's tooltip says **Backup paused**. While backups run, the tooltip names them ("Backup running: Data and Music"). Windows notifications report finished backups and restores.

While the GUI is not connected to the background service (it has not asked yet, you declined, or it let go of the token after 15 minutes in the tray: [connecting](#connecting-to-the-background-service)), the tray does not name Backup Sets. Its tooltip says what the service reports to anyone: how many backups are running or waiting ("2 backups running, 1 waiting"), "Last backup failed: open the app" when the last run of a Backup Set failed, or when the last backup ended. **Pause backup** then reads **Pause backup...**: it opens the window and asks for administrator rights to connect first, then pauses the service's backups.

**Linux has no tray.** Closing the window quits the app, and **Minimize to Tray** does nothing. Use the background service for schedules that run while the app is closed.

### Windows service

With the background service installed and running, the service runs the schedules and backups started from the GUI are handed to it, so scheduled Backup Sets run even when nobody is signed in and the GUI is closed. The service runs as Local System, so VSS works without starting the GUI as administrator. Encryption keys used through the service must be set to **Remember on this computer**. Without the service, the GUI runs the schedules itself while it is open. The MSI installs and starts the service for you; if it has been removed, install it again here.

**Run as Service** (Preferences > Advanced) shows whether the service is installed, whether it is running, how it starts ("starts with Windows"), and whether the GUI is handing its work to it ("This app hands backups and schedules to the service." or "This app is running the schedules itself."). As administrator you can:

| Button | What it does |
|---|---|
| Install and start | Installs `salvanza-svc.exe`, which must be in the same folder as `salvanza-gui.exe`, as an automatic Windows service named `salvanza-svc`, running as Local System, restarted automatically if it stops unexpectedly. If the service has no settings yet, your own (servers, Backup Sets, history and Message Log) are copied to it first; your own folder is left as it was, and key files stay where they are. Then starts it. The GUI stops its own scheduler and hands schedules and backups to the service, so nothing runs twice |
| Start / Stop | Starts or stops the installed service. While it is stopped nothing runs the computer's Backup Sets: the GUI does not run them itself, and says that the service is not running, with **Start the service** ([connecting](#connecting-to-the-background-service)), until it is back. Stop is refused while the service is running a backup |
| Remove | Stops and uninstalls the service. Its settings stay in `C:\ProgramData\Salvanza\`; the GUI works on your own settings again and runs their schedules while it is open |

If the GUI is not running as administrator, a **Restart as administrator** button is shown. If the installed service runs a program other than the one beside this GUI, the tab says which. A service installed before the rename to Salvanza (`ProxmoxBackupClientSVC`, or `ProxmoxBackupClient` from an early test build) is recognised too. **Install and start** does not reuse such a service: it installs `salvanza-svc`, and the new service removes the earlier one, and another brand's service, when it starts.

### Linux service

The .deb, .rpm and Arch packages install the service program (`/usr/libexec/salvanza/salvanza-svc`) and the systemd unit `salvanza.service`. A fresh install enables and starts the service, as the Windows installer does; an upgrade keeps your choice, so a service you turned off stays off. **Run as Service** (Preferences > Advanced, in the GUI started as administrator) turns it on again:

| Button | What it does |
|---|---|
| Install and start | Copies your Backup Sets, servers, keys and history to `/var/lib/salvanza/` (only if nothing is there yet, and no earlier install's settings in `/var/lib/proxmoxbackupclient/` are there to take over; your own `~/.salvanza/` is left as it was), then enables and starts the unit, which runs as root and starts with the computer. From then on the service keeps the settings, the GUI works on them through the service ([connecting](#connecting-to-the-background-service)), and it hands schedules and backups to the service |
| Start / Stop | `systemctl start` / `stop`. While it is stopped nothing runs the computer's Backup Sets: the GUI does not run them itself, and says that the service is not running, with **Start the service** ([connecting](#connecting-to-the-background-service)), until it is back. Stop is refused while the service is running a backup |
| Remove | Stops and disables the unit. The settings stay in `/var/lib/salvanza/` for when the service is turned on again; the GUI works on your own settings again |

`/var/lib/salvanza/` belongs to root alone (folders 0700, files 0600), as `C:\ProgramData\Salvanza\` belongs to SYSTEM and Administrators. A GUI started normally works on the settings through the service once it has connected ([below](#connecting-to-the-background-service)). Earlier versions let the `sudo` (or `wheel`) group read the folder directly; the service undoes that when it starts. Backups you start from the GUI run in the service, which records them in Reports as it does its scheduled runs. An upgrade of the package restarts a running service. Removing the package stops and disables the service and keeps `/var/lib/salvanza/`; delete that folder yourself to remove the settings. Without systemd there is no service and the GUI runs the schedules while it is open.

Runs the service makes on its own schedule show in an open GUI window, with Pause, Resume and Stop ([section 8](#8-the-backup-progress-card)); several at once each have their own. The service runs backups side by side as the app does, with the same **Maximum Concurrent Backups** setting ([section 6.7](#67-several-backups-at-once)), which it reads before each run.

### The service program from the command line

The service program also installs and controls itself:

```
salvanza-svc.exe -service install|uninstall|start|stop|restart
```

Run it as administrator (Windows) or root (Linux). Usually you do not need to:

* **Windows.** The installer (MSI) installs the service as `salvanza-svc` (a branded build's as its own program name, such as `nimbusbackup-svc`), and **Preferences > Advanced > Run as Service** in the GUI installs, starts, stops and removes it under the same name, and so does `-service install`. **Run as Service** also copies your own settings (`%USERPROFILE%\.salvanza`) to `C:\ProgramData\Salvanza` if the service has none yet; `-service install` does not. The services of earlier installs (`ProxmoxBackupClientSVC`, `ProxmoxBackupClient` from test builds before 14 October 2026, and the other brands' services) are stopped and removed by the new service when it starts, so there are never two.
* **Linux.** The `.deb`, `.rpm` and Arch packages install the systemd unit `salvanza.service`, and a fresh install enables and starts it; an upgrade keeps your choice, so a service you turned off stays off. To turn it on again, use **Run as Service** in the GUI started as administrator: it copies your Backup Sets, servers, keys and history to `/var/lib/salvanza`, where the service reads them, then enables and starts the unit. `systemctl` and `/usr/libexec/salvanza/salvanza-svc -service ...` act on the same unit but do not copy any settings.

The service's settings folder is the service's alone: SYSTEM and Administrators on Windows, root (0700, files 0600) on Linux. The GUI reaches it through the service's local API on `127.0.0.1:18765`, which needs the token in `api-token` in that folder. The GUI gets the token by asking for administrator rights (UAC, or pkexec on Linux) and keeps it in memory. The service makes a new token each time it starts, and every 24 hours.

### Connecting to the background service

With the background service installed, the service alone keeps the computer's settings: the PBS servers and their tokens, Backup Sets, Reports, the Message Log and the keys folder. The GUI shows and changes them through the service, and for that it needs the service's token, which only administrators can read. So the GUI asks for administrator rights once to get it: a UAC prompt on Windows, a password prompt (pkexec) on Linux. On a standard account, give an administrator's name and password.

The GUI does not ask when it starts in the tray at sign-in (if the installer was told to start it then). It asks the first time its window is in front of you: when you open it from the tray or the Start menu, or click into it. Until you agree, or if you decline, the window shows "Not connected to the background service. It keeps this computer's PBS servers and Backup Sets; connecting asks for administrator rights." with a **Connect** button. While it is not connected the GUI shows no servers or Backup Sets and cannot change any. After a decline it does not ask again by itself: click **Connect** when you are ready.

The GUI keeps the token in memory only, and only while its window is in use. While it holds the token, another program running as you could read it from the GUI's memory and use the service, which runs as Local System or root. So once the window has been hidden in the tray for 15 minutes (**Minimize to Tray**, or started in the tray at sign-in), the GUI lets go of the token and is no longer connected; opened again within the 15 minutes, it keeps it. The service also replaces its token each time it starts and every 24 hours. Either way the window then shows **Connect** again and asks once more the next time it comes to the front. Requests the GUI makes in the background never ask, and the service goes on running the Backup Sets. A GUI running as administrator or as root lets go of the token too, and connects again without asking.

Before it sends anything, the GUI checks on each connection that the program answering is the background service itself (on Windows the service's own process, on Linux a program run as root), so another program listening in its place while the service is stopped never gets the token or your settings; the GUI then behaves as if the service were not running.

**The service is not running.** While the service is installed but stopped, the window says "The background service is not running, so this computer's Backup Sets do not run on their schedules." instead of offering **Connect**, and **Add New Backup Set** and **One-off Backup** stay greyed out. **Start the service** starts it with one administrator prompt, which also connects the GUI (no prompt when the GUI already runs as administrator or as root); if you decline, or the service does not start, the banner says so. The GUI notices within a few seconds when the service stops or starts again, and on Windows the tray icon's tooltip says so too.

**Use my own settings.** A user who cannot answer the prompt (no administrator's password to hand) can click **Use my own settings** beside **Connect**. The GUI then works on that user's own PBS servers and Backup Sets, kept in their user folder (`~/.salvanza`), as it does on a computer without the service. Those Backup Sets run only while the app is open or minimised to the tray: the background service does not run them when the app is closed or the user is signed out. A banner says so, with **Connect** to go back to the computer's settings. The choice is remembered for that user. The computer's own Backup Sets carry on in the service either way.

**The computer's servers on your own settings.** On your own settings, **Preferences > Account Information** also offers **Use one of this computer's servers**: a list of the PBS servers the computer already has, by name, address and datastore. Choosing one opens the add-server form filled in with its name, address, datastore, namespace and certificate fingerprint, on the login tab the computer uses, for your own API token or user name and password. The computer's login, token secret, password and encryption key are never shared: if the computer encrypts its backups to that server, the list says so, and you choose your own key under **Encryption**. The button is greyed out, with the reason as its tooltip, while the background service is not running or the computer has no servers.

**Connect with your own Backup Sets.** Clicking **Connect** while the GUI works on your own settings first says how many Backup Sets you have of your own, that they do not run while connected, and that they are kept and come back with **Use my own settings**. **Connect and copy my sets to this computer** also adds them, with their PBS servers, to the computer's settings, where the service runs them whether or not anyone is signed in. A server the computer already has (same address, datastore, namespace and login) is used as it is; a server or Backup Set whose name or ID is taken gets your user name added; an encryption key is copied into the service's `keys` folder. The computer's default server, e-mail account and theme are not changed, and your own copy is left as it was.

A GUI already running as administrator (after **Restart as administrator**, or started with **Run as administrator**) or as root reads the token itself and connects without asking. Otherwise the GUI stays a normal program while connected: only a small helper is elevated to fetch the token, so mapped network drives and drag and drop from Explorer keep working.

For a restore, browsing a backup or a search, the service hands the GUI only what that one operation needs: a fresh login ticket for a server with a user name and password, or the API token for a server with a token, and the encryption key when the backup is encrypted. The GUI keeps them in memory only.

Schedules run in the service whether the GUI is connected or not, also when nobody is signed in.

On Linux, connecting needs pkexec (polkit): the .deb recommends it, and the .rpm and the Arch package require it.

## 16. Compatibility with the official Proxmox client

This client and the official `proxmox-backup-client` read each other's backups. Tested with `proxmox-backup-client` 3.4.9 and PBS 4.2:

* Legacy, Data and Metadata snapshots made by this client (GUI and command line, Windows and Linux) restore byte-identically with the official client, and PBS 4.2's file browser opens them.
* Legacy, Data and Metadata snapshots made by the official client restore byte-identically with this client, fully or selectively.
* Both use the same split archive format (pxar version 2) and the same encryption key file format, so keys work in both.

This client adds two things to its folder snapshots, and the official client restores the files as normal without them:

* `.proxmox_backup_client_meta.json` at the root, which records the original path. The official client restores it as an ordinary file.
* `proxmox-client-acls.json.gz.blob`, which records the permissions and other file details: on Windows the NTFS permissions, attributes, creation times, alternate data streams and extended attributes; on Linux the extended attributes and POSIX ACLs. The official client and the PBS web page leave it out; this app's Restore puts them back.

On Linux the official client keeps POSIX ACLs, extended attributes and file capabilities inside the archive itself. This client does not write or read those entries yet, so on Linux ACLs do not carry between the two clients in either direction; see the [roadmap](../../TODO.md).

## 17. Troubleshooting

**A USB or external source disk drops out during a backup.** The backup waits up to 2 minutes for the disk to come back, reopens the file and carries on, logging both events. If one file still cannot be read, only that file is affected: it is filled with zeros to its recorded size, marked incomplete in the log, and read again on the next run. If the disk does not come back within 2 minutes the backup fails. A recurring drop-out usually means a cable, port or hub problem; avoid chaining USB hubs.

**A backup stays at "Creating VSS snapshot..." (Windows).** Another shadow copy is being made, by another backup program, System Restore or a backup of this app that timed out. The backup tries again every 30 seconds for up to 10 minutes, then fails with "another shadow copy was still being made after 10m0s". It never deletes other shadow copies or restarts the VSS service; `vssadmin list shadows` and `vssadmin list writers` show what VSS is doing.

**A machine backup ends as partial.** Part of a disk could not be read (bad sectors). That part is zeros in the image; the card, the Reports row and the log say where and how much. Check the disk's health.

**Listing snapshots takes a long time.** PBS reads its datastore directory to answer. On a datastore that is cold (spun-down disks) or busy with a verify, garbage collection or another backup this can take a minute or more. The GUI waits up to 120 seconds and shows the slow-server hint after 15.

**A PBS verify job fails on an empty snapshot folder.** A backup that was interrupted at the wrong moment can leave a snapshot folder on PBS with no `index.json.blob`. Verify reports it as "manifest load error ... No such file or directory" and the job ends as failed, even though every real snapshot verified. Delete that snapshot in the PBS web interface (Datastore > Content), or remove the empty folder on the PBS host.

**Restore says the owner of some files stayed the restoring account (Windows).** Setting a file's owner to another account needs the "Restore files and directories" privilege, which the GUI switches on when it runs as administrator. Run as administrator, any recorded owner comes back, even an account this computer does not know. Without it the permissions are still restored and only the owner differs; the restore log lists those files.

**Every Metadata run reads every file.** Check that no other set or computer uses the same Backup ID on the same server and namespace (look for a **SHARED ID** badge, [section 6.4](#64-destination-tab)), and that the encryption key has not changed. The stage line says why the run is reading every file.

**Scheduled backups fail with a passphrase message.** The key is set to **Ask me** and nobody has entered the passphrase since the GUI started, or the run went through the service. Use **Remember on this computer** for unattended runs.

**A restore says "this snapshot was encrypted with another key".** The key on the server's **Encryption** tab is not the one the snapshot was made with. The message gives the fingerprint of the key the snapshot needs and of the key in use; bring the right one in with **Import...** (the paper key or a copy of the key file). With no key set, it says the snapshot needs its encryption key and gives that key's fingerprint. Nothing is restored or overwritten before this check.

**"Not connected to the background service."** The GUI has not had administrator rights yet in this session, or you declined. Click **Connect** and allow the prompt; on a standard account, give an administrator's name and password ([section 15](#connecting-to-the-background-service)).

**Machine backup on Linux fails with a permission or snapshot error.** With the background service, the service runs it as root; without it, use **Restart as administrator** in the app. Load `elastio-snap` or `dattobd` first ([installing them](#linux-snapshots-installing-elastio-snap-or-dattobd)).

**Restore of an older client version shows folders without files.** That computer has a version without split-archive support. Update it.

**Hard links.** When a file has two names (a hard link), only one name is restored.

**A restore of many small files is slow (Windows).** Restoring many small files takes longer than a few large ones of the same total size. Most of the time per file goes on Windows Defender (or another virus scanner) checking each new file, and then on putting back its permissions; writing several files at once was measured and gave no gain. Restoring without **Restore permissions/ACLs**, when you do not need them, saves part of it.
