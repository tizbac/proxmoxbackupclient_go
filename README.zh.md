# Proxmox Backup Client：Proxmox Backup Server 的 Windows 和 Linux 客户端

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client 是一个面向 Proxmox Backup Server (PBS) 的开源 (GPL-3.0) 备份客户端，可在 Windows 和 Linux 上运行。**

它是一套用于备份到 PBS 的**工具**：

- **Proxmox Backup Client GUI**（基于 RDEM Systems 的 Nimbus Backup GUI）：图形客户端，可将 Windows 和 Linux 服务器及工作站备份到 PBS。支持一致性快照（Windows 上用 VSS，Linux 上用 elastio-snap/dattobd）、带计划的 Backup Sets、文件夹备份和整盘备份、恢复、Undelete 和 Roll back、多台 PBS 服务器，以及 Windows 和 Linux 上的后台服务。
- **`proxmoxbackup-directory`**：文件夹 (pxar) 备份命令行工具，带去重和变更检测。
- **`proxmoxbackup-machine`**：对 Windows (VSS) 和 Linux (elastio-snap/dattobd) 机器做在线整盘备份 (fidx) 的命令行工具；裸机恢复后镜像可直接启动。
- **`proxmoxbackup-nbd`**：把磁盘备份挂接为 NBD 设备用于恢复 (Linux)。
- 基于 Clonezilla Live 的**裸机恢复 ISO**。

> ⚠️ **免责声明：** 本项目与 **Proxmox Server Solutions GmbH** **没有任何关联**。“Proxmox”、Proxmox 徽标及相关名称归其各自所有者所有，此处**仅**用于说明兼容性。其产品请见 [proxmox.com](https://www.proxmox.com/)。

> ℹ️ **关于本页：** 这是英文 README 的中文摘要译本。**[英文 README.md](README.md) 是权威版本**，包含全部细节。用户手册只有英文版（见下表）。应用界面目前没有中文，因此本文中的菜单、标签页和按钮名称保留应用里的英文原文。

## 📖 用户手册

| 手册 | 内容 |
|---|---|
| **[GUI 手册](docs/manual/GUI.md)** | 安装、PBS 服务器、Backup Sets 和计划、增量备份、进度、恢复、Undelete、Roll back、Reports、日志、Preferences、后台服务、故障排除 |
| **[命令行手册](docs/manual/CLI.md)** | `proxmoxbackup-directory`、`proxmoxbackup-machine`、`proxmoxbackup-nbd`：所有参数、配置文件、退出码、计划任务；命令行下的 Undelete 和 Roll back |
| **[裸机恢复手册](docs/manual/BMR.md)** | 恢复 ISO 分步说明；把 `vm` 快照恢复到 Proxmox VE；不同硬件 |
| [恢复场景](docs/RESTORE_GUIDE.md) | 什么情况下用哪种恢复 |
| [手册索引](docs/manual/README.md) | 三本手册概览 |

## 🆕 master 分支上，下一个版本将包含

以下内容记录在 [CHANGELOG](CHANGELOG.md) 的 [Unreleased] 下，尚未正式发布：

- **安装包**：**Windows MSI**（应用和后台服务装在 Program Files 下，有开始菜单项，支持原地升级，卸载时可选择保留或删除设置），以及 **Linux 软件包** `.deb`（Debian、Ubuntu、Mint）和 `.rpm`（Fedora），包含应用、三个命令行工具、手册和菜单项。
- **Linux 后台服务**（`proxmoxbackupclient.service`）：计划中的 Backup Sets 以 root 运行，无人登录时也会执行。与 Windows 一样，在 **Preferences > Advanced > Run as Service** 中开启。**Restart as administrator** 现在在 Linux 上也可用（通过 pkexec）。
- 由服务运行的备份也可 **Pause、Resume、Stop**，打开应用时能看到其进度卡片。
- 两个 Backup Sets 共用同一备份组（相同服务器、类型和 Backup ID）时会**发出警告**，两张卡片都显示 **SHARED ID** 标记。
- **设置目录加锁**：Windows 上 `C:\ProgramData\ProxmoxBackupClient` 仅对管理员开放；Linux 上设置目录仅属主可访问。
- **从所有常见文件系统的整机备份中取回文件**：除 NTFS 外还支持 FAT、exFAT、ext2/3/4 和 XFS。Windows 镜像恢复同时还原权限和备用数据流。
- **Linux 整机备份跳过空闲空间**（ext2/3/4、XFS），与 Windows 自 v0.8.0 起相同。
- Linux 整机 Backup Sets 支持 **Undelete 和 Roll back**；任何 Backup Set 也可**在命令行中**使用：`-list-sets`、`-undelete`、`-rollback`（见[命令行手册](docs/manual/CLI.md)第 12 节）。
- 命令行工具把错误输出到 stderr，不再弹出消息框；`proxmoxbackup-machine` 会发送通知邮件。

## 💽 v0.8.0 新功能：更快的整机备份、从中取回文件、暂停/继续

- Windows 上**整机备份跳过空闲空间**（以及分页文件和休眠文件），半空的磁盘读取时间约减半；遇到**坏扇区**也不再中断，该处以零写入镜像，并在运行结果中注明位置。
- 整机备份进度卡片显示块进度、磁盘和分区、New / Reused / 空白块数量以及上传速率。
- **从整机备份中恢复文件**：镜像里的每个 NTFS 卷都是可浏览、搜索和勾选的文件夹。无需先挂载或下载，只读取文件表和所选文件的数据（一台 127 GB、43 万个文件的笔记本 C: 盘在 4 到 15 秒内列出）。所以每台笔记本一份整机备份，既能裸机恢复，也能找回单个文件。
- **Undelete 和 Roll back** 可用于整机 Backup Sets (Windows)，作用于为该备份集选定的文件夹（默认是 Desktop、Documents、Pictures、Music、Videos 和 Downloads）。
- 正在运行的文件夹或整机备份可 **Pause 和 Resume**；每张备份集卡片有 **VSS / Snapshot ON-OFF 标记**；Windows **创建时间**会被备份和还原。
- **v0.8.2：** 空白磁盘空间不再逐块计算哈希，大部分为空的磁盘只需原来约三分之一的时间（50 GB 测试盘：3 分 36 秒变为 1 分 15 秒）。

详见 [CHANGELOG](CHANGELOG.md)。

## 🗑️ v0.7.0 新功能：Undelete 和 Roll back

GUI 为文件夹 Backup Sets 新增两个页面：

- **Undelete** 查找仍在备份中但磁盘上已不存在的文件（范围可选最新一次备份，或最近 7 天、30 天的所有备份），以文件夹树显示每个文件何时消失（同一文件出现在别处时提示“probably moved to ...”），并把勾选的文件恢复到原位置或其他文件夹。在一个 76.1 万个文件的共享上删除 39 GB 后，7 天范围的搜索约一分钟就准确找出了这些文件。
- **Roll back** 把备份集的文件夹恢复到所选备份时的状态。先预览会有哪些变化（缺失、已更改、之后新增），再执行恢复。被替换或删除的每个文件都会先移到同一磁盘的一旁，所以 **Undo** 可以全部复原。

只读取备份的文件列表和文件夹的目录列表，恢复之前不打开任何文件。同时新增：应用内确认对话框、每张 Backup Set 卡片上的下次运行时间、为每个备份集建议的 Backup ID（如 `deepthought-data`）、唯一的备份集名称，以及 Windows 下载包内含后台服务（Preferences > Advanced > Run as Service）。见 [GUI 手册](docs/manual/GUI.md#10-undelete)和 [CHANGELOG](CHANGELOG.md)。

## ⚡ v0.6.0 新功能：增量文件夹备份

文件夹备份现在可以跳过未更改的文件，使用与官方 `proxmox-backup-client` 相同的**变更检测**。在 **Metadata** 模式下，客户端把每个文件的大小、修改时间（精确到纳秒）、权限模式和属主与上一个快照比较。未更改的文件根本不打开，其数据直接复用上一个快照的块，只读取并上传新增和修改的文件。

在一台真实文件服务器上的测量（480.9 GB，USB 磁盘上 761,866 个文件）：

| 运行 | 耗时 | 读取 | 上传的新块 |
|---|---|---|---|
| 首次 Metadata 运行（无可比较的快照） | 4 小时 17 分 | 全部文件 | 135,549 |
| 之后的运行 | 约 10 到 15 分钟（距上次运行仅几分钟时更快） | 只读已更改文件；761,866 个未更改文件全部复用 | 4 到 40 |

未更改的数据既不读取也不发送，因此到远程或异地 PBS 的流量也只剩实际变化的部分。

用法：编辑文件夹 **Backup Set**，在 **Destination** 标签页设置 **Change detection: Metadata**。设置 **Read every file every N runs**，让每第 N 次运行做一次完整读取，以捕捉大小和时间都没变的少见修改（例如每 4 小时运行一次的备份集设为 42，约每周一次）。每个备份集请使用**独立的 Backup ID**：Metadata 模式与组内最新快照比较，两个备份集共用 ID 会互相导致全量读取。命令行：`-change-detection-mode metadata`。默认仍为 Legacy；详见下文“更快的文件夹备份”一节和 [GUI 手册](docs/manual/GUI.md#7-incremental-folder-backups-change-detection)。

## 🤝 与官方 Proxmox 客户端双向兼容

已用 `proxmox-backup-client` 3.4.9 和 PBS 4.2 验证：

- **本客户端生成的 Legacy、Data、Metadata 快照可由官方客户端逐字节一致地恢复**，无论通过 GUI 还是命令行、在 Windows 还是 Linux 上创建，PBS 4.2 的文件浏览器也能打开。在 Linux 上，恢复的文件还保留精确的权限模式、纳秒级修改时间和符号链接目标。
- **官方客户端生成的 Legacy、Data、Metadata 快照可由本客户端逐字节一致地恢复**，全部或选择性恢复均可。
- 相同的拆分归档格式 (pxar version 2)、相同的变更检测模式、相同的加密密钥文件格式（`proxmox-backup-client key create` 生成的密钥可在这里使用，反之亦然）、相同的 `PBS_REPOSITORY` 类环境变量。

因此备份不会被绑定在本客户端上。唯一的额外内容是本客户端在文件夹快照根目录添加的一个小文件 `.proxmox_backup_client_meta.json`（记录原始路径），官方客户端会把它当作普通文件恢复。

## 🔧 关于本 fork

这是上游项目的 **mjb-is fork**，仓库：**https://github.com/mjb-is/proxmoxbackupclient_go**。目标是让 PBS 成为家庭实验室里唯一的服务器端备份系统，取代目前混用的多种工具；同时加固 Windows/Linux GUI 客户端，并在 Clonezilla 之上做一条全自动裸机恢复路径。相对上游的主要改进：重建的 GUI（菜单栏、侧边菜单、Backup Sets、Reports、Message Log、五种配色主题和自定义颜色、18 种界面语言）、修复 PBS/NBD 取块时的 `zstd.NewReader` 死锁（[PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)）、全自动 “PBS Bare Metal Restore” 向导、恢复时还原 NTFS ACL 和 POSIX ACL/xattr 以及可选的恢复后校验、更完善的 Backup Set 工作流、把整机备份存为 Proxmox VE `vm` 快照等。

通用的改进都会以 PR 形式提交上游。目前本 fork 已有 17 个 PR 被原项目合并，另有一个 ([#105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105)) 待合并。完整列表见[英文 README](README.md#-about-this-fork)。

## 📦 下载和安装

👉 **[下载最新版本](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| 你的系统 | 使用 |
|---|---|
| Windows | **MSI 安装包**（`ProxmoxBackupClient-<version>-x64.msi`）把应用和后台服务装在 Program Files 下，并添加开始菜单项。也可以解压 `ProxmoxBackupClient-v<version>-windows-amd64.zip`（应用和 `ProxmoxBackupClientSVC.exe`，两者需放在同一文件夹） |
| Debian、Ubuntu、Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| 其他 Linux | 解压 `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz`（需要 WebKitGTK 运行库） |
| 仅命令行 | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` 或 `-linux-amd64.tar.gz` |

MSI、`.deb` 和 `.rpm` 是 master 上的新内容：v0.8.1 及以前的版本只有 zip 和 tar.gz。在正式版本附带安装包之前，请按 [BUILD.md](BUILD.md) 自行构建。

Linux 软件包会安装两个菜单项：**Proxmox Backup Client** 以当前用户运行（备份自己的文件夹）；**Proxmox Backup Client (administrator)** 通过 polkit 询问密码后以 root 运行，用于整机备份和块快照。后台服务已安装但默认未启用，见 [packaging/debian/README.md](packaging/debian/README.md)。

> ⚠️ **Windows 提示“检测到病毒”（如 `Trojan:Win32/Sabsik.FL.A!ml`）或 SmartScreen 警告？**
> 这是 Go/Wails 应用的**已知误报**，*不是*病毒。`!ml` 后缀表示机器学习模型的判定，它会标记*未签名且不常见*的可执行文件。上游有更多说明：[原因以及如何验证下载](https://github.com/tizbac/proxmoxbackupclient_go)。

### 🔎 验证下载

本 fork 的每个版本都附带 `SHA256SUMS.txt`：

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Linux 上：`sha256sum -c SHA256SUMS.txt --ignore-missing`。

> ℹ️ 本 fork 自己的构建**尚无代码签名、CI 签名证明或 VirusTotal 扫描**。它们是手动构建和上传的。如果你愿意帮忙搭建，请看 [TODO.md](TODO.md)。

## ✨ 功能

### GUI：Proxmox Backup Client GUI（推荐）
- **🌍 多语言**：18 种界面语言（法语、英语、意大利语、德语、波兰语、西班牙语、保加利亚语、捷克语、希腊语、匈牙利语、拉脱维亚语、立陶宛语、荷兰语、葡萄牙语、罗马尼亚语、斯洛伐克语、土耳其语和乌克兰语）
- 原生菜单栏和侧边菜单（Backup、Restore、Undelete、Roll back、Reports、Message Log、Preferences、About）
- **Backup Sets**：可复用的命名任务，按每日 / 间隔 / 手动调度，带目录或整机类型标记、VSS / Snapshot 标记、“Run Now”（Metadata 备份集可选快速或完整读取），两个备份集写入同一备份组时显示 **SHARED ID** 警告
- 文件夹或整盘的**一次性备份**
- **Reports** 页面记录完整的备份和恢复历史，**Message Log** 用于诊断（有容量上限）
- **Preferences** 对话框：五种配色主题（Amber、Blue、Green、Red、Dark）和自定义颜色
- **增量文件夹备份**（变更检测，Metadata 模式）：与官方客户端一样，未更改的文件不读取直接复用，可选每 N 次运行做一次完整读取
- Windows 和 Linux 上的**整机（整盘）备份**：跳过空闲空间，遇到坏扇区不中断，可存为 `host` 或 Proxmox VE `vm` 快照
- **恢复**：浏览快照、搜索文件（通配符、正则表达式、完整路径）、选择性恢复，还原 NTFS/POSIX ACL、文件夹时间戳、Windows 创建时间，可选校验
- **从整机备份中取回文件**：浏览并恢复磁盘镜像中 NTFS 卷上的文件（master 上还支持 FAT、exFAT、ext2/3/4 和 XFS）
- **Undelete** 和 **Roll back**（自 v0.7.0）：找出备份里还有但磁盘上已没有的文件并放回；或把备份集的文件夹恢复到所选备份时的状态，先预览、事后可 Undo。适用于文件夹和整机备份集。见 [GUI 手册](docs/manual/GUI.md#10-undelete)
- 正在运行的备份可 **Pause、Resume、Stop**
- **Run as Service**：在 Preferences > Advanced 中安装、启动、停止或移除后台服务，无人登录时计划也能运行（Windows 自 v0.7.0；Linux 在 master 上）
- 实时进度卡片：开始时间和预计完成时间、已完成文件数/总数、处理速度和实际上传速度、当前文件，以及本次为何读取全部文件
- Windows 上的 VSS（卷影复制），Linux 上的 elastio-snap/dattobd 块快照
- 邮件通知和备份后操作、Backup Set 克隆、设置导出和导入
- 多台 PBS 服务器，证书指纹固定（首次使用即信任），API 令牌或用户名/密码登录
- **客户端加密**：每台服务器一个 PBS 密钥文件（AES-256-GCM，清单签名），可选口令保护
- 修改 Backup Sets 需管理员权限 (Windows)，带 Restart as administrator
- 用于诊断的调试日志

### 命令行工具
- `proxmoxbackup-directory`：文件夹 (pxar) 备份，带去重和 `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`：对 Windows (VSS) 和 Linux (elastio-snap/dattobd) 机器做在线整盘备份（fidx，增量，除非指定 `-readfreespace` 否则跳过空闲空间）
- `proxmoxbackup-nbd`：用于恢复磁盘备份的 NBD 服务器 (Linux)
- `ProxmoxBackupClientSVC` (Windows) 和 `proxmoxbackupclient`（Linux 软件包）：对 GUI 中定义的 Backup Sets 使用 `-list-sets`、`-undelete` 和 `-rollback`（master 上）

前三个工具都接受 `-keyfile` 和 `-keyfile-passphrase` 用于加密快照。完整参考见[命令行手册](docs/manual/CLI.md)。

### 📸 截图

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**：命名任务，带重复/手动触发标记和目录/整机类型标记，每个都有 Run Now

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**实时备份进度**：吞吐量、预计剩余时间、新块与复用块数量

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**恢复**：选择 PBS 服务器和 Backup ID，列出快照或按文件名跨快照搜索

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**作为 Proxmox VE 虚拟机的 Backup Sets**：存为 `vm` 快照的备份集带 `MACHINE (VM)` 标记

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**选择性恢复**：在快照树中勾选文件/文件夹，恢复到原位置或其他位置

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**：完整运行历史和详情面板（耗时、块、已备份的文件夹）

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**多 PBS**：管理多台 PBS 服务器，测试连接，选择默认服务器

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**添加/编辑服务器**：URL、用户名/密码或 API 令牌、数据存储和命名空间

</td>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**主题**：五种内置配色和自定义颜色，实时为整个应用换色

</td>
</tr>
</table>

更多截图（Message Log、高级选项、全部主题、多语言界面）见[英文 README](README.md#-screenshots)。

### 智能系统排除（文件夹备份）
文件夹备份会自动排除：

**系统文件夹：** 任何位置的 `System Volume Information`（VSS 存储，可达 100+ GB）和 `$RECYCLE.BIN`；仅驱动器根目录下的 `Recovery`（master 上；早期版本会跳过任意层级的同名文件夹）；`.pbs-rollback`（Roll back 的安全文件夹）。
**系统文件：** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys` 和 `DumpStack.log.tmp`。

**为什么重要：** 一个驱动器可能显示已用 1.03 TB，而真实文件只有约 141 GB。不排除的话备份会包含 VSS 快照（浪费空间和时间）；排除后大小与真实数据相符。

**建议：** 文件级备份用带这些排除的**文件夹**备份；裸机恢复用单独 Backup Set 中的**整机**备份（整盘镜像，空闲空间、分页文件和休眠文件以零写入）。

### 安全与质量
- 可选的客户端逐块加密，清单签名
- 设置目录在 Windows 上仅对管理员开放，在 Linux 上仅属主可访问（master 上）
- 本地服务 API 只监听 127.0.0.1，需要令牌，每次服务启动时更新
- 输入验证和凭据清理
- 路径遍历防护
- 指数退避重试
- 每个模块都有错误处理和单元测试，另有可选的实时测试和针对真实 PBS 的端到端测试

## 🔐 客户端加密

快照可以在离开机器前加密，使用 Proxmox Backup Server 密钥文件（即 `proxmox-backup-client key create` 生成的格式，因此密钥与官方客户端通用）。每个块用 AES-256-GCM 加密，清单签名，PBS 服务器永远看不到密钥。

**GUI：** 打开 **Preferences > Account Information**，编辑服务器，进入 **Encryption** 标签页。选择已有的密钥文件，或点 **Generate...** 生成新密钥（高级选项可用口令保护）。Reports 会显示每次备份是否加密。**没有密钥文件就无法恢复快照：请在本机之外的安全位置保存一份副本。**

**带口令的密钥：** **Ask me** 每个应用会话询问一次，只保存在内存中，计划任务和后台服务运行的备份会直接失败并给出明确提示；**Remember on this computer** 把口令像 PBS 令牌一样存在配置中，适合无人值守的计划任务和后台服务。

**命令行：**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

**裸机恢复：** Clonezilla ISO 会询问密钥（无、U 盘上的密钥文件或手动输入路径）及口令，密钥不会写入 ISO。见 [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)。注意：`config.json` 以明文保存 PBS 令牌、密码和记住的口令；已知限制详见[英文 README](README.md#-client-side-encryption)。

## 更快的文件夹备份：变更检测

默认的 Legacy 模式每次都读取所有文件。Backup Sets（和命令行）可以改用官方 `proxmox-backup-client` 的变更检测模式：

| 模式 | 读取 | 存储形式 |
|---|---|---|
| **Legacy**（默认） | 所有文件 | 每个文件夹一个 `<name>.pxar.didx`，与以前相同 |
| **Data** | 所有文件 | 拆分归档：`<name>.mpxar.didx`（元数据）+ `<name>.ppxar.didx`（文件内容） |
| **Metadata** | 仅新增和修改的文件 | 同样的拆分归档；大小、修改时间（纳秒）、权限模式和属主与上次备份一致的文件直接复用，不读取 |

Metadata 模式首次运行读取全部内容，之后只读取变化部分。大小和修改时间都不变的修改不会被发现，因此备份集可设置**每 N 次运行读取所有文件**。只有加密方式和密钥相同的上一次备份才能被复用。

**GUI：** 编辑 Backup Set，**Destination** 标签页，**Change detection**。只适用于文件夹备份集；整机备份不受影响。一次性备份使用 Legacy。**每个备份集一个 Backup ID。**

**命令行：** `-change-detection-mode legacy|data|metadata`，或 JSON 配置中的 `"change-detection-mode"`。

**切换到 Data 或 Metadata 之前**，请把所有需要从中恢复的机器升级到 v0.6.0 或更高版本：旧版本无法读取拆分归档。

## 🚀 快速开始 (GUI)

1. 安装：Windows 用 MSI，Linux 用 `.deb` 或 `.rpm`，或从[发布页](https://github.com/mjb-is/proxmoxbackupclient_go/releases)解压 zip 或 tar.gz
2. 启动。Windows 上，VSS、整机备份和修改 Backup Sets 需要**以管理员身份运行**。Linux 上，备份自己的文件夹以普通用户即可；整机备份和块快照请用 **Proxmox Backup Client (administrator)**（或以 root 运行）
3. 在 **Preferences > Account Information** 中添加 PBS 服务器并点 **Test**
4. 创建 Backup Set（或一次性备份），选择要备份的文件夹或磁盘
5. 运行。要让计划在无人登录时也执行，开启 **Preferences > Advanced > Run as Service**

[GUI 手册](docs/manual/GUI.md)逐步讲解每个步骤。

## 整机在线备份

`proxmoxbackup-machine`（以及 GUI 的整机备份）可在不停机的情况下备份完整的 Windows 10/11、Windows Server 或 Linux 系统。命令语法与文件夹备份基本相同，只是用 `-backupdev` 代替 `-backupdir`（多块磁盘可重复）。例如：

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

该命令检查磁盘 0，找出所有已挂载分区，对其创建 VSS 快照，然后把整盘生成可启动的 fidx 备份镜像。空闲空间、分页文件和休眠文件以零写入镜像而不读取（master 上 Linux 的 ext2/3/4 和 XFS 空闲空间也一样）；`-readfreespace` 则读取全部内容，用于取证副本。之后的备份是增量的，哈希计算已并行化，可轻松达到 1 GB/秒。

### Linux 整机备份前提条件（VSS 的替代）

Linux 没有 VSS，本项目使用 `elastio-snap` 内核模块（`dattobd` 的 fork）对正在写入的块设备做一致的时间点快照。原 `elastio/elastio-snap` 仓库已归档，请安装维护中的 [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)。如果 `dattobd` 已加载，也同样支持。

以下两条缺一不可，否则 Linux 整机备份会拒绝运行：

1. **必须安装并加载内核模块。** 官方仓库包的安装方式（见[其 INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)）至少在 Ubuntu 22.04 上链接已失效（2026-09-24 确认），从源码构建则很可靠：
   ```bash
   sudo apt-get install linux-headers-$(uname -r) build-essential   # Debian/Ubuntu
   git clone --depth 1 https://github.com/Axcient/elastio-snap.git
   cd elastio-snap
   sudo make
   sudo make install
   sudo depmod -a     # required - `make install` only copies the .ko, it never
                       # runs depmod, so a bare `modprobe elastio-snap` right
                       # after a fresh install fails with "not found in
                       # directory" even though the file is right there
                       # (confirmed live 2026-09-24)
   sudo modprobe elastio-snap
   lsmod | grep elastio-snap   # confirms it loaded
   ```
   这只对本次启动有效；如需开机自动加载，请参考发行版文档（例如 `/etc/modules-load.d/` 条目）。

2. **备份进程本身必须以 root (EUID 0) 运行。** 即使模块已加载，`CreateVSSSnapshot`（尽管名字偏向 Windows，它就是 Linux 的快照路径）也会拒绝以非 root 用户运行，加入 `disk` 组之类的方式无法替代。整机备份请使用 **Proxmox Backup Client (administrator)** 菜单项、后台服务，或用 `sudo`（或 root）运行 GUI 或 `proxmoxbackup-machine`。普通文件/目录备份没有此要求。

如果整机备份报错 `backup device /dev/sdX: open /dev/sdX: permission denied`，通常只是需要把用户加入 `disk` 组（`sudo usermod -aG disk $USER`，然后重新登录），但这本身不足以做真正的快照，紧接着仍会遇到上面的 root 要求。

### 从磁盘备份中恢复文件

GUI 的 Restore 页面像打开文件夹备份一样打开整机备份：镜像中的每个卷都是可浏览、可恢复的文件夹（NTFS 自 v0.8.0；master 上还有 FAT、exFAT、ext2/3/4 和 XFS），不需要挂载。

在 Linux 上也可以用 nbd 工具把磁盘备份挂接为块设备。先执行 `modprobe nbd max_part=0`（原因不明，`max_part != 0` 会导致无限的分区探测循环）。NBD 工具可以连接任何固定磁盘备份，无论是 VM 还是 host 备份（也适用于 PVE 备份）。命令类似：

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

省略 `-path` 会出现终端界面供你选择 fidx 文件。

> 不要在运行重要服务的机器上使用（损坏的文件系统可能导致操作系统崩溃，这也是 Proxmox VE 为此使用 QEMU 实例的原因）。

停止 pbsnbd 之前务必卸载 nbd 磁盘上的所有内容，否则很可能留下一个忙碌、无法卸载的分区。如果出现 `Device or resource busy` 错误，需要运行 `nbd-client -d /dev/nbd0` 强制断开，或直接重启。

### 恢复到物理机

本 fork 提供打过补丁的 Clonezilla Live ISO，带全自动的 **“PBS Bare Metal Restore”** 启动菜单项（见下文“Clonezilla Live ISO”一节）：它位于首位并在超时后默认启动，跳过 Clonezilla 的语言/键盘提示。输入 PBS 连接信息、选择快照、确认自动检测到的目标磁盘后，网络设置、NBD 挂接、恢复本身和自动重启都无需再操作。

较早的半手动方式也仍在同一 ISO 上，作为 Clonezilla 常规模式选择菜单中的 “pbs-nbd” 项：它引导你输入 PBS 连接信息，把所选快照在后台挂接到 `/dev/nbd0`，然后由你通过 Clonezilla 自带的磁盘恢复向导手动完成恢复。

## 用法：目录备份

完整参考（含 Windows 和 Linux 示例、退出码和计划任务）见 **[命令行手册](docs/manual/CLI.md)**。典型命令：

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackup-directory
  -baseurl string            PBS URL, example: https://192.168.1.10:8007
  -certfingerprint string    Certificate fingerprint (asked for interactively when omitted)
  -authid string             API token ID (user@realm!token)
  -secret string             API token secret
  -pbsusername string        PBS user for ticket login (with -pbspassword; overrides -authid/-secret)
  -pbspassword string        Password for -pbsusername (asked for when omitted)
  -datastore string          Datastore name
  -namespace string          Namespace (optional)
  -backup-id string          Backup ID (default: the hostname)
  -backupdir string          Source directory; repeat for several (each becomes group <id>_<path>)
  -change-detection-mode     legacy (default), data or metadata
  -novss                     Do not snapshot the source (VSS on Windows, elastio-snap/dattobd on Linux)
  -keyfile string            PBS encryption key file
  -keyfile-passphrase string Passphrase for a protected key (asked for when omitted)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

也会读取官方客户端的 `PBS_REPOSITORY`、`PBS_SERVER`、`PBS_PORT`、`PBS_DATASTORE`、`PBS_AUTH_ID`、`PBS_PASSWORD` 和 `PBS_FINGERPRINT` 环境变量。提供了 JSON 配置示例，只需填写需要的字段。邮件主题和正文使用 [Go 模板引擎](https://pkg.go.dev/text/template)；可用变量（`.NewChunks`、`.ReusedChunks`、`.Status` 等）见[英文 README](README.md#usage-directory-backup)。

**流备份：** `-backupstream` 可以备份数据流而不是 pxar，例如 `mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]`。

**已知问题：** 启用 Windows Defender 实时防护会使备份最多慢 25%。Windows 上命令行工具有进程锁，第二个备份会打印 “Another backup job is running” 并退出；Linux 上没有此锁，不要同时安排两个命令行运行。

## 📋 环境要求

- Windows 10/11 或 Windows Server（64 位），VSS 和整机备份需要管理员权限
- 或 Linux（软件包适用于 Debian、Ubuntu、Mint 和 Fedora），GUI 需要 WebKitGTK 运行库，一致的整机备份需要 root 以及 elastio-snap 或 dattobd 内核模块
- 能访问 Proxmox Backup Server 的网络

## 🔨 从源码构建

**[BUILD.md](BUILD.md)** 有完整说明：GUI 和后台服务、命令行工具、MSI 以及 Linux 软件包。简要来说，需要 Go 1.25 或更高版本和 Wails CLI v2.13：

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

已构建的前端 (`gui/frontend/dist`) 已提交到仓库，只有修改前端时才需要 Node.js。测试：`make test`。

## 🔧 高级用法与指南

### 多 PBS（多台 PBS 服务器）

配置多台 PBS 服务器，为每个 Backup Set 或一次性备份选择目标（例如 `C:\Users` 每天备份到快速 SSD PBS，`D:\` 每周备份到大容量 PBS，另加一台灾备服务器）。

- **[用户指南](MULTI_PBS_USER_GUIDE.md)**：添加和测试服务器、默认服务器、为每个备份选择服务器、常见问题和故障排除。
- **[实现指南](MULTI_PBS_GUIDE.md)**：数据模型、从单 PBS 配置自动迁移、后端方法。

旧的单 PBS 配置在首次加载时自动迁移为 `default` 服务器。

### Clonezilla Live ISO（裸机恢复）

救援流程通过给标准 Clonezilla Live ISO 打补丁来构建，加入 `pbsnbd` / `machinebackup` 二进制文件和两个菜单项（支持 CD 启动、用 `dd` 写入 USB 以及 UEFI）：

- **`ocs-pbs-nbd`**：在 Clonezilla 模式选择菜单中添加的 **“pbs-nbd”** 项，挂接快照后交给 Clonezilla 的常规磁盘恢复向导手动操作。
- **`ocs-pbs-bare-metal-restore`**：新的首位默认启动项 **“Proxmox Backup Client Go - PBS Bare Metal Restore”**，从输入 PBS 凭据一路到完成恢复并自动重启：先尝试 DHCP（失败才提示静态 IP），默认同步 NTP，自动检测目标磁盘。Windows 和 Linux 备份同样适用。

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

完整细节见 **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**。

### 构建 Windows GUI

**原生 Windows：** 工具链（Go、MinGW、Wails）以及应用、服务和 MSI 的构建命令见 **[BUILD.md](BUILD.md)**。**Docker（在 Linux 上构建时）：** `./build_gui_windows_docker.sh` 在一次性 `golang` 容器中构建 `ProxmoxBackupClientGO.exe`，不构建后台服务和 MSI。

### 功能状态、更新日志和内部文档

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**：已完成、部分完成或未完成的功能，以及从哪个版本开始。
- **[CHANGELOG.md](CHANGELOG.md)**：按版本的变更历史，[Unreleased] 为 master 上的内容。
- **[TODO.md](TODO.md)**：开放的路线图和想法。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**：每个版本包含的内容。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**：MSI 卸载对话框（保留/删除配置）及其测试计划。
- **[AGENTS.md](AGENTS.md)**：仓库结构和约定，供贡献者和编码代理参考。

## 🌐 意大利技术支持

本项目及其**所有**贡献都将永远公开，并以 GPLv3 许可。本项目及最新整机备份功能的主要赞助方 E.T.I. Srl ( https://etitech.net ) 可为有需要的用户提供 Proxmox 部署，特别是 Windows 备份工具方面的支持。

永远不会有“社区版”和“企业版”之分，技术支持只是独立的服务。你提出的任何定制，即使开发需要付费，也会像整个项目一样以 GPLv3 发布。

## 🖥️ GUI 归属

**Proxmox Backup Client GUI** 基于 **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**，由 **[RDEM Systems](https://www.rdem-systems.com/)** 开发和维护。

该 GUI（最初是本项目的 fork）已合并回本仓库：包括 GUI 及其所有功能在内的全部代码继续以 GPLv3 开源。RDEM Systems 赞助 GUI 开发并提供商业支持。

## ⚠️ 警告

本软件按“原样”提供。尽管我们追求可靠，但对数据丢失或损坏不承担任何责任。在生产环境中依赖备份之前，请务必测试备份并验证恢复。

本软件仍处于 alpha 质量阶段，我们对任何损坏或数据丢失（包括源文件）不承担责任。

## 📄 许可证

GPLv3：见 [LICENSE](LICENSE) 文件。

## 🏷️ 品牌

凡贡献了**至少 5 个**增加功能或修复问题的 commit 的贡献者，都有权添加自己的品牌信息用于商业用途。

唯一条件是品牌所指向的公司**没有**从事以下任何活动：

- 恶意软件活动
- 宣扬战争的业务（适用于任何国家，包括西方国家）
- 诈骗
- 数据窃取
- 拐卖儿童
- 暴力
- 歧视
- 毒品
- 任何被普遍认为违法的活动

如果有针对任何贡献者的投诉，我们会尝试联系；如果没有合理解释，将**立即终止**该权益。

**GPLv3 许可证仍然有效**，你仍可以自由 fork 本项目并构建自己的可执行文件。

## 🤝 贡献

GUI 已完全实现，但仍欢迎贡献，尤其是：

1. 加密：由母语者审校机器翻译的加密相关文字，以及密钥文件之外的密钥管理方案（例如 Windows DPAPI 或凭据管理器）
2. 物理机到虚拟机 (P2V) 迁移，把裸机备份恢复为虚拟机（通过 `vm` 快照类型已能完成基本功能，已用 Windows UEFI 磁盘验证；仍缺少网卡、TPM、virtio 驱动、取自真实机器的 CPU/内存设置以及非 SATA 磁盘总线）
3. 块的异步上传 / 多核上传（整机备份已实现多核压缩）
4. Proxmox 端补丁，为 pxar 格式添加一种包含 Windows 安全描述符的新条目
5. 支持 Windows 符号链接
6. 任何你能想到的有趣想法 :)

## 关于 Proxmox Backup Client GO 贡献者

Proxmox Backup Client GO 贡献者开发并维护本项目。该软件依赖 NTP/NTS 基础设施，以及社区参考中列出的 [11 个公共 NTS 服务器](https://github.com/jauderho/nts-servers)。

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
