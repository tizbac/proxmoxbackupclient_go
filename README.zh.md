# Proxmox Backup Client：Proxmox Backup Server 的 Windows 和 Linux 客户端

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client 是一个面向 Proxmox Backup Server (PBS) 的开源 (GPL-3.0) 备份客户端，可在 Windows 和 Linux 上运行。** 它把文件夹备份为可浏览的文件归档，把整台机器备份为可启动的磁盘镜像，可以按计划运行，也可以随时手动运行；恢复时小到单个文件、大到整台机器都能取回，可以恢复到原来的电脑、替换用的电脑或 Proxmox VE 虚拟机上。

我为自己的家庭实验室 (homelab) 开发它，目的是让 PBS 成为 Windows 和 Linux 服务器、台式机和笔记本混合环境中唯一的备份系统，取代 Backup for Workgroups、rsync 镜像以及 Clonezilla/RescueZilla 镜像。它于 2026 年 9 月作为 [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) 的 fork 起步，如今已远远超出原项目：此后已有 350 多个 commit，其中 18 项修复和功能已回馈给原项目（[见下方列表](#-对原项目的贡献)）。

**欢迎按原样使用。** 它正式定位为 **alpha 软件**，使用风险完全由你自己承担，不提供任何形式的保证或担保。每个版本都由我亲自测试，我自己的机器也在日常使用它，但在依赖它之前，请先阅读[状态、错误报告和功能请求](#-状态错误报告和功能请求)。

本套件包括：

- **Proxmox Backup Client GUI**：Windows 和 Linux 上的应用。Backup Sets（备份集），每个可设多个计划；带一致性快照的文件夹备份和整盘备份（Windows 上用 VSS，Linux 上用 elastio-snap/dattobd）；从文件夹备份和整机备份中恢复文件；Undelete 和 Roll back；多台 PBS 服务器；客户端加密；以及后台服务，使计划在无人登录时也能运行。
- **`proxmoxbackup-directory`**：命令行文件夹 (pxar) 备份，带去重和变更检测。
- **`proxmoxbackup-machine`**：命令行在线整盘备份 (fidx)，适用于 Windows 和 Linux 机器；恢复后镜像可以直接启动。
- **`proxmoxbackup-nbd`**：把磁盘备份挂接为 NBD 设备 (Linux)。
- 基于 Clonezilla Live 的**裸机恢复 ISO**，从开机到重启全程引导完成恢复。

> 关键词：Proxmox 备份客户端 Windows · PBS 客户端 · Windows VSS 备份 · 物理机转虚拟机 (P2V) · 裸机恢复 · Proxmox Backup Server 图形界面。

## ⭐ 功能一览及实测结果

| | 结果 |
|---|---|
| **增量文件夹备份** | 一个 480.9 GB、761,866 个文件的文件服务器共享：首次运行 4 小时 17 分，之后每次 10 到 15 分钟。未更改的文件根本不会被打开（即官方客户端的 Metadata 变更检测）。 |
| **快速整机备份** | 不读取空闲空间、分页文件和休眠文件，空白磁盘空间也不计算哈希。一块已用 373 GB 的 1 TB 笔记本磁盘：以前需要 1 小时 23 分到 1 小时 44 分，现在 38 分钟。一块 50 GB 测试盘：以前 3 分 36 秒，现在 1 分 15 秒。 |
| **从整机备份中取回文件** | 像浏览文件夹一样浏览磁盘镜像并恢复单个文件，支持 NTFS、FAT、exFAT、ext2/3/4 和 XFS。一台笔记本 127 GB、43 万个文件的 C: 盘在 4 到 15 秒内列出；无需先挂载或下载任何东西。 |
| **物理机转为虚拟机** | 备份一台 PC 或服务器，然后把它作为 Proxmox VE 虚拟机启动：可以通过 Proxmox VE 本身恢复，也可以用恢复 ISO。[见下文](#-物理机转为虚拟机-p2v)。 |
| **裸机恢复** | 启动 ISO（U 盘、光盘或 PXE），输入 PBS 信息，选择备份：一台 32 GB 的 Windows 机器在开机 8 分钟后就回到了登录界面。 |
| **Undelete** | 在一个 76.1 万个文件的共享上删除 39 GB 后，7 天范围的搜索约一分钟就准确找出了这些文件，并把它们放回原处。 |
| **Roll back** | 把备份集的文件夹恢复到所选备份时的状态，先预览，事后可 Undo。 |
| **计划** | 每个 Backup Set 可设多个，从每隔几分钟到每年一次：“每天 02:00”再加“每月最后一天”或“第 2 个星期二”，每个都用通俗文字显示，并列出接下来的 3 次运行。 |
| **恢复保真度** | Windows 上：NTFS 权限、属性、备用数据流和扩展属性、创建时间和文件夹时间；Linux 上：权限模式、属主、ACL、扩展属性和符号链接。可选的校验步骤会重新读取已恢复的内容并与备份比较。 |
| **兼容 Proxmox 自家工具** | 这里生成的备份可以用官方 `proxmox-backup-client` 逐字节一致地恢复，反之亦然。[详情](#-与官方-proxmox-客户端双向兼容)。 |

## 🧩 相对原项目新增的功能

除了[已合并到原项目的 18 项改动](#-对原项目的贡献)之外，本客户端还增加了：

**备份**
- 增量文件夹备份，采用官方客户端的变更检测（Data 和 Metadata 模式，拆分的 pxar v2 归档），并可每 N 次运行做一次完整读取。
- 整机备份跳过空闲空间、分页文件和休眠文件（Windows NTFS；Linux ext2/3/4 和 XFS），把无法读取的扇区写为零并继续运行，运行时显示块、磁盘、分区和 chunk 计数。
- 文件夹备份和整机备份均可 Pause、Resume 和 Stop，后台服务运行的备份也可以。
- Windows 文件夹备份包含备用数据流、NTFS 扩展属性和创建时间。

**计划和 Backup Sets**
- 每个 Backup Set 可设多个计划，从每隔几分钟到每年一次（每月第 n 个星期几、每月最后一天），每个都用通俗文字描述，并列出接下来的 3 次运行。
- Linux 和 Windows 上都有后台服务，Linux 上也可以 Restart as administrator。
- 快速或完整的 Run Now、VSS / Snapshot 标记、SHARED ID 警告、备份集名称作为快照备注写入 PBS、邮件提醒、备份后操作，以及设置导出和导入。

**恢复**
- 从整机备份中取回单个文件（NTFS、FAT、exFAT、ext2/3/4、XFS），无需挂载或下载镜像。
- Undelete 和 Roll back，适用于文件夹和整机 Backup Sets，在 GUI 和命令行中均可使用。
- 恢复后可选的校验步骤、Windows 创建时间，以及按文件夹逐级加载的快照树，因此非常大的备份也能立即打开。
- 引导式裸机恢复 ISO，也能恢复到空虚拟机中；以及经过测试的物理机转虚拟机路线（[P2V](#-物理机转为虚拟机-p2v)）。
- 与官方 `proxmox-backup-client` 双向逐字节兼容，包括其 Data 和 Metadata（拆分归档）快照。

**应用**
- 重建的界面：菜单栏、侧边菜单、记录完整备份和恢复历史的 Reports、Message Log、五种配色主题和自定义颜色、托盘通知和弹出通知。
- 18 种界面语言，每一种都经过审校（原项目有 5 种）。
- 设置目录仅限管理员 (Windows) 或其属主 (Linux) 访问，服务 API 令牌在每次启动时更新。
- MSI、`.deb` 和 `.rpm` 安装包，以及 GUI、命令行和裸机恢复的用户手册。

## 🔄 物理机转为虚拟机 (P2V)

物理 PC 或服务器的整机备份可以通过两种方式变成 Proxmox VE 虚拟机：

**1. 在 Proxmox VE 中恢复。** 把 Backup Set（或一次性备份）设为 **Backup as: Proxmox VE virtual machine**。备份以 `vm/<number>` 加一个小型 VM 配置的形式保存，因此 Proxmox VE 会把它和其他 VM 备份列在一起：在 Proxmox VE 中打开 PBS 存储，选择该备份，点 **Restore**，选择 VM ID 和存储。该 VM 会得到一块 SATA 磁盘以及与磁盘匹配的固件（UEFI/GPT 用 OVMF，BIOS/MBR 用 SeaBIOS）；启动前请添加一块网卡；如果 UEFI 虚拟机无法启动，再添加一块 EFI 磁盘（不勾选 Pre-Enroll keys）。

**2. 用裸机恢复 ISO 恢复到空虚拟机中。** 创建一台固件与原机器相同的 VM（UEFI/GPT 用带 EFI 磁盘的 OVMF，BIOS/MBR 用 SeaBIOS），配一块至少与原磁盘一样大的 SATA 磁盘和一块 Intel E1000 网卡，从 ISO 启动并运行恢复。这对 `host` 和 `vm` 备份同样适用，而且同一份备份仍然可以恢复到裸机上。

已测试的路线：

| 从 | 到 | 路线 | 结果 |
|---|---|---|---|
| Windows 10 笔记本，UEFI/GPT | Proxmox VE VM | 恢复 ISO | 启动后直接进入用户桌面 |
| Windows Server，UEFI/GPT | Proxmox VE VM | Proxmox VE 恢复 | 无需任何修改即可启动 |
| Xubuntu 台式机，UEFI/GPT | Proxmox VE VM | Proxmox VE 恢复，以及恢复 ISO | 启动到桌面 |
| Linux，BIOS/MBR | Proxmox VE VM | 恢复 ISO，以及 Proxmox VE 恢复 | 启动到登录提示 |
| Linux Mint，加密备份 | Proxmox VE VM | 恢复 ISO，密钥来自 U 盘 | 已启动 |
| Linux PC | 换了新磁盘的同一台 PC | 恢复 ISO | 已启动 |

Windows 在首次启动时会配置新的虚拟硬件（“正在准备设备”，英文版为 “Getting devices ready”）。[裸机恢复手册](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines)提供固件对照表、驱动和分步说明。

## 📖 用户手册

| 手册 | 内容 |
|---|---|
| **[GUI 手册](docs/manual/GUI.md)** | 安装、PBS 服务器、Backup Sets 和计划、增量备份、进度、恢复、Undelete、Roll back、Reports、日志、Preferences、后台服务、故障排除 |
| **[命令行手册](docs/manual/CLI.md)** | `proxmoxbackup-directory`、`proxmoxbackup-machine`、`proxmoxbackup-nbd`：所有参数、配置文件、退出码、计划任务；命令行下的 Undelete 和 Roll back |
| **[裸机恢复手册](docs/manual/BMR.md)** | 恢复 ISO 分步说明；把 `vm` 备份恢复到 Proxmox VE；物理机转虚拟机；不同硬件 |
| [恢复场景](docs/RESTORE_GUIDE.md) | 什么情况下用哪种恢复 |
| [手册索引](docs/manual/README.md) | 三本手册概览 |

## 🆕 v0.9.0 新功能：每个 Backup Set 可设多个计划、安装包、Linux 服务

- **每个 Backup Set 可设多个计划**，频率从每隔几分钟到每年一次：“每天 02:00”再加“每月最后一天”或“第 2 个星期二”，每个计划都用通俗文字描述，并列出接下来的 3 次运行。现有备份集保留原来的计划。
- **文件夹备份保留备用数据流和 NTFS 扩展属性**（Windows），例如“从互联网下载”标记，恢复时一并还原。已知限制列表现已清空。
- **安装包**：**Windows MSI**（应用和后台服务装在 Program Files 下，有开始菜单项，支持原地升级，卸载时可选择保留或删除设置），以及 **Linux 软件包** `.deb`（Debian、Ubuntu、Mint）和 `.rpm`（Fedora），包含应用、三个命令行工具、手册和菜单项。
- **Linux 后台服务**（`proxmoxbackupclient.service`）：计划中的 Backup Sets 以 root 运行，无人登录时也会执行。与 Windows 一样，在 Preferences > Advanced > Run as Service 中开启。**Restart as administrator** 现在在 Linux 上也可用（通过 pkexec）。
- **由服务运行的备份也可 Pause、Resume 和 Stop**，打开应用时能看到其进度卡片。
- **两个 Backup Sets 共用同一备份组时发出警告**（相同服务器、类型和 Backup ID），两张卡片上都显示 **SHARED ID** 标记。
- **设置目录加锁**：Windows 上 `C:\ProgramData\ProxmoxBackupClient` 现在仅对管理员开放；Linux 上设置目录仅属主可访问。
- **从所有常见文件系统的整机备份中取回文件**：除 NTFS 外，还支持 FAT、exFAT、ext2/3/4 和 XFS。Windows 镜像恢复同时还原权限和备用数据流。
- **Linux 整机备份跳过空闲空间**（ext2/3/4、XFS），与 Windows 自 v0.8.0 起的做法相同。
- **Linux 整机 Backup Sets 支持 Undelete 和 Roll back**，任何 Backup Set 也可**在命令行中**使用：`-list-sets`、`-undelete`、`-rollback`（见[命令行手册](docs/manual/CLI.md)第 12 节）。
- 命令行工具把错误输出到 stderr，不再弹出消息框；`proxmoxbackup-machine` 会发送通知邮件。
- **裸机恢复 ISO** 不会把自己启动所用的 U 盘列为恢复目标磁盘；手册还说明了把物理机变成虚拟机时如何匹配固件（UEFI 或 BIOS）。
- **所有语言均已审校**（17 种翻译，约 1,850 条字符串），日期和数字格式跟随应用语言。

早期版本：**v0.8** 更快的整机备份（跳过空闲空间、遇到坏扇区不中断）、从整机备份中取回文件、Pause 和 Resume；**v0.7** Undelete 和 Roll back；**v0.6** 增量文件夹备份。所有改动见 [CHANGELOG](CHANGELOG.md)。

## 🤝 与官方 Proxmox 客户端双向兼容

已用 `proxmox-backup-client` 3.4.9 和 PBS 4.2 验证：

- **本客户端生成的 Legacy、Data、Metadata 快照可由官方客户端逐字节一致地恢复**，无论通过 GUI 还是命令行、在 Windows 还是 Linux 上创建，PBS 4.2 的文件浏览器也能打开它们。在 Linux 上，恢复的文件还保留精确的权限模式、纳秒级修改时间和符号链接目标。
- **官方客户端生成的 Legacy、Data、Metadata 快照可由本客户端逐字节一致地恢复**，全部恢复或选择性恢复均可。
- 相同的拆分归档格式 (pxar version 2)、相同的变更检测模式、相同的加密密钥文件格式（`proxmox-backup-client key create` 生成的密钥可在这里使用，这里生成的密钥也可在那边使用）、相同的 `PBS_REPOSITORY` 类环境变量。

因此你的备份永远不会被绑定在本客户端上：Proxmox 自家的工具能恢复本客户端生成的每一个快照，本客户端也能恢复官方客户端生成的每一个快照。唯一的额外内容是本客户端在文件夹快照根目录添加的一个小文件 `.proxmox_backup_client_meta.json`（记录原始路径），官方客户端会把它当作普通文件恢复。

> ⚠️ **免责声明：** 本项目与 **Proxmox Server Solutions GmbH** **没有任何关联**。“Proxmox”、Proxmox 徽标及相关名称归其各自所有者所有，此处**仅**用于说明兼容性。其产品请见 [proxmox.com](https://www.proxmox.com/)。

## 📦 下载和安装

👉 **[下载最新版本](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| 你的系统 | 使用 |
|---|---|
| Windows | **MSI 安装包**（`ProxmoxBackupClient-<version>-x64.msi`）把应用和后台服务装在 Program Files 下，并添加开始菜单项。也可以解压 `ProxmoxBackupClient-v<version>-windows-amd64.zip`（应用和 `ProxmoxBackupClientSVC.exe`，两者需放在同一文件夹） |
| Debian、Ubuntu、Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| 其他 Linux | 解压 `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz`（需要 WebKitGTK 4.1 运行库） |
| 仅命令行 | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` 或 `-linux-amd64.tar.gz` |
| 需要恢复的机器 | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`，即裸机恢复 ISO |

MSI、`.deb` 和 `.rpm` 是 v0.9.0 新增的：v0.8.1 及以前的版本只有 zip 和 tar.gz 文件。如需自行构建，请参阅 [BUILD.md](BUILD.md)。

Linux 软件包会安装两个菜单项：**Proxmox Backup Client** 以当前用户身份运行应用（备份你自己的文件夹）；**Proxmox Backup Client (administrator)** 通过 polkit 询问你的密码后以 root 运行，用于整机备份和块快照。后台服务已安装但默认未启用，见 [packaging/debian/README.md](packaging/debian/README.md)。

> ⚠️ **Windows 提示“检测到病毒”（如 `Trojan:Win32/Sabsik.FL.A!ml`）或 SmartScreen 警告？**
> 这是 Go/Wails 应用的**已知误报**，*不是*病毒。`!ml` 后缀表示机器学习模型的判定，它会标记*未签名且不常见*的可执行文件。请用 `SHA256SUMS.txt` 核对下载的文件（见下文）。

### 🔎 验证下载

每个版本都附带 `SHA256SUMS.txt`：

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Linux 上：`sha256sum -c SHA256SUMS.txt --ignore-missing`。

> ℹ️ **目前还没有代码签名、CI 签名证明或 VirusTotal 扫描。** 版本是手动构建和上传的，还没有通过能生成可验证构建来源证明的 CI 流水线。如果你愿意帮忙搭建，请看 [TODO.md](TODO.md)。

## ✨ 功能

### GUI
- **🌍 多语言**：18 种界面语言：法语、英语、意大利语、德语、波兰语、西班牙语、保加利亚语、捷克语、希腊语、匈牙利语、拉脱维亚语、立陶宛语、荷兰语、葡萄牙语、罗马尼亚语、斯洛伐克语、土耳其语和乌克兰语
- 原生菜单栏和侧边菜单（Backup、Restore、Undelete、Roll back、Reports、Message Log、Preferences、About）
- **Backup Sets**：可复用的命名任务，**每个可设多个计划**（从每隔几分钟到每年一次，或手动），带目录或整机类型标记、VSS / Snapshot 标记、“Run Now”（Metadata 备份集可选快速或完整读取），两个备份集会写入同一备份组时显示 **SHARED ID** 警告
- 文件夹或整盘的**一次性备份**
- **Reports** 页面记录完整的备份和恢复历史，另有用于诊断、有容量上限的 **Message Log**
- **Preferences** 对话框：五种配色主题（Amber、Blue、Green、Red、Dark）和自定义颜色选项
- **增量文件夹备份**（变更检测，Metadata 模式）：与官方客户端一样，未更改的文件不读取直接复用，可选每 N 次运行做一次完整读取
- Windows 和 Linux 上的**整机（整盘）备份**：跳过空闲空间，遇到坏扇区不中断，可存为 `host` 备份或 Proxmox VE `vm` 备份
- **物理机转虚拟机**：整机备份可以恢复为 Proxmox VE 虚拟机，通过 Proxmox VE 或恢复 ISO 均可
- **恢复**：浏览快照、搜索文件（通配符、正则表达式、完整路径）、选择性恢复，还原 NTFS/POSIX ACL、备用数据流、文件夹时间戳、Windows 创建时间，并可选校验步骤
- **从整机备份中取回文件**：浏览并恢复磁盘镜像中 NTFS、FAT、exFAT、ext2/3/4 和 XFS 卷上的文件
- **Undelete** 和 **Roll back**：找出备份集的备份里还有、但磁盘上已经没有的文件并放回；或把备份集的文件夹恢复到所选备份时的状态，先预览，事后可 Undo。适用于文件夹和整机备份集。见 [GUI 手册](docs/manual/GUI.md#10-undelete)
- 正在运行的备份可 **Pause、Resume 和 Stop**
- **Run as Service**：在 Preferences > Advanced 中安装、启动、停止或移除后台服务，无人登录时计划也能运行（Windows 和 Linux）
- 实时进度卡片：开始时间和预计完成时间、已完成文件数/总数、处理速度（每秒数据量和文件数）和实际上传速度、当前文件，以及本次为何读取全部文件
- Windows 上的 VSS（卷影复制），Linux 上的 elastio-snap/dattobd 块快照
- 邮件通知和备份后操作、Backup Set 克隆、设置导出和导入
- 多台 PBS 服务器，证书指纹固定（首次使用即信任），API 令牌或用户名/密码登录
- **客户端加密**：每台服务器一个 PBS 密钥文件（AES-256-GCM，清单签名），可选口令保护，见[客户端加密](#-客户端加密)
- 修改 Backup Sets 需要管理员权限，并提供 Restart as administrator
- 用于诊断的调试日志

### 命令行工具
- `proxmoxbackup-directory`：文件夹 (pxar) 备份，带去重和 `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`：对 Windows (VSS) 和 Linux (elastio-snap/dattobd) 机器做在线整盘备份（fidx，增量，除非指定 `-readfreespace` 否则跳过空闲空间，`-type vm` 生成 Proxmox VE 备份）
- `proxmoxbackup-nbd`：用于恢复磁盘备份的 NBD 服务器 (Linux)
- `ProxmoxBackupClientSVC` (Windows) 和 `proxmoxbackupclient`（Linux 软件包）：对 GUI 中定义的 Backup Sets 使用 `-list-sets`、`-undelete` 和 `-rollback`

前三个工具都接受 `-keyfile` 和 `-keyfile-passphrase`，用于加密快照。完整参考见[命令行手册](docs/manual/CLI.md)。

### 📸 截图

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**：命名任务，带下一个要运行的计划的标记（DAILY、WEEKLY、MONTHLY，其他计划显示 +N）和目录/整机类型标记，每个都有 Run Now

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**实时备份进度**：运行中的吞吐量、预计剩余时间、新块与复用块数量

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**恢复**：选择 PBS 服务器和 Backup ID，列出快照或按文件名跨快照搜索

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**计划**：每个备份集可设多个，从每隔几分钟到每年一次（“每月最后一天”“第 2 个星期二”），并列出接下来的 3 次运行

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**备份为 Proxmox VE VM**：选择虚拟机类型和数字 ID；输入时即显示 PBS 的 Backup ID

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**作为 Proxmox VE VM 的 Backup Sets**：存为 `vm` 备份的备份集带 `MACHINE (VM)` 标记

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**一次性备份**：一次性备份表单上的同一选项

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**选择性恢复**：在快照树中勾选单个文件/文件夹，恢复到原位置或其他位置

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**：完整运行历史和详情面板（耗时、块、已备份的文件夹）

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Message Log**：有容量上限、带时间戳的诊断信息流，涵盖所有备份和恢复

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**多 PBS**：管理多台 PBS 服务器，测试连接，选择默认服务器

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**添加/编辑服务器**：URL、用户名/密码或 API 令牌、数据存储和命名空间

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**主题**：五种内置配色和一个自定义颜色，实时为整个应用换色（见下方图条）

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**备份选项**：如实说明：并行恢复解包属于实验功能，在我们自己的测试硬件上实测反而*更慢*

</td>
</tr>
</table>

**🎨 配色主题**：每次选择后，Preferences 对话框本身就会换色，而不只是一个色块：

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Amber

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Classic Blue

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Green

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Red

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Dark

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Custom

</td>
</tr>
</table>

**🌍 多语言**：法语、意大利语和西班牙语的 Reports 页面（另有 15 种，包括英语、德语和波兰语）：

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-lang-french.png" width="280">](docs/screenshots/gui-lang-french.png)
Français

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-italian.png" width="280">](docs/screenshots/gui-lang-italian.png)
Italiano

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-spanish.png" width="280">](docs/screenshots/gui-lang-spanish.png)
Español

</td>
</tr>
</table>

### 智能系统排除（文件夹备份）
文件夹备份会自动排除以下内容：

**系统文件夹：** 任何位置的 `System Volume Information`（VSS 存储，可达 100+ GB）和 `$RECYCLE.BIN`；仅驱动器根目录下的 `Recovery`；`.pbs-rollback`（Roll back 的安全文件夹）。
**系统文件：** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys` 和 `DumpStack.log.tmp`。

**为什么重要：** 一个驱动器可能显示已用 1.03 TB，而真实文件只有约 141 GB。不排除的话，备份会包含 VSS 快照（浪费空间和时间）；排除后，大小与真实数据相符。

**建议：** 文件级备份请用带这些排除项的**文件夹**备份；裸机恢复和 P2V 请在单独的 Backup Set 中使用**整机**备份（它是整盘镜像；空闲空间、分页文件和休眠文件以零写入）。

### 安全与质量
- 可选的客户端逐块加密，清单签名
- 设置目录在 Windows 上仅对管理员开放，在 Linux 上仅属主可访问
- 本地服务 API 只监听 127.0.0.1，需要令牌，令牌在每次服务启动时更新
- 输入验证和凭据清理
- 路径遍历防护
- 指数退避重试逻辑
- 每个模块都有错误处理和单元测试，另有可选的实时测试和针对真实 PBS 的端到端测试

## 🔐 客户端加密

快照可以在离开机器之前加密，使用 Proxmox Backup Server 密钥文件（即 `proxmox-backup-client key create` 写出的格式，因此密钥与官方客户端通用）。每个块都用 AES-256-GCM 加密，清单经过签名。PBS 服务器永远看不到密钥。

**GUI：** 打开 **Preferences > Account Information**，编辑服务器，进入其 **Encryption** 标签页。选择已有的密钥文件，或点 **Generate...** 生成新密钥（Advanced 选项可用口令保护它）。该标签页显示密钥指纹。Reports 会显示每次备份是否加密。**没有密钥文件就无法恢复快照：请在本机之外的安全位置保存一份副本。**

**带口令保护的密钥（scrypt 或 PBKDF2）：** 选择应用如何解锁密钥：
- **Ask me**：每个应用会话询问一次口令，只保存在内存中。计划备份和启动时的备份从不弹出询问：在该会话中输入口令之前，它们会立即失败并给出明确提示。对 Ask 类密钥，通过后台服务运行的备份也会立即失败。
- **Remember on this computer**：口令像 PBS 令牌一样存在配置中，因此任何能读取 `config.json` 的人都能解锁密钥。无人值守的计划任务和后台服务应选择这一项。

选择加密快照进行恢复时，会询问口令。

**命令行：**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` 只在密钥受保护时需要，省略时会提示输入。`proxmoxbackup-machine` 和 `proxmoxbackup-nbd` 接受同样的两个参数；`proxmoxbackup-nbd` 需要它们来挂接加密的磁盘镜像，并在使用 NBD 设备之前先检查第一个块，因此密钥错误会立即失败。

**裸机恢复：** 恢复 ISO 会询问密钥（无、U 盘上的密钥文件或手动输入路径），以及受保护密钥的口令。密钥永远不会写入 ISO。见 [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)。

**已知限制**
- GUI 生成的密钥以受保护的 ACL 写入（仅当前用户、SYSTEM 和 Administrators）。你放在其他位置的密钥文件，其权限取决于所在文件夹。
- `config.json` 以明文保存 PBS 令牌、密码和任何记住的口令。从 v0.9.0 起，其所在目录仅对管理员 (Windows) 或仅对属主 (Linux) 开放；在 v0.8.2 及以前，Windows 上本地 Users 可读取它，因此在共用机器上请自行限制访问。
- 恢复加密快照需要同一个密钥文件；密钥丢失后无法恢复。
- Ask 密钥流程和服务是不同的进程：凡是通过服务运行的任务，请使用 Remember。

## 更快的文件夹备份：变更检测

在默认的 Legacy 模式下，每次文件夹备份都会读取所有文件，所以即使什么都没变，一次运行也要花读取整个文件夹的时间。Backup Sets（和命令行）可以改用官方 `proxmox-backup-client` 的变更检测模式。Metadata 模式已在一台 480.9 GB、761,866 个文件的服务器上日常使用，变化很少的运行只需 10 到 15 分钟，而不是 4 个多小时。

| 模式 | 读取 | 存储形式 |
|---|---|---|
| **Legacy**（默认） | 所有文件 | 每个文件夹一个 `<name>.pxar.didx`，与以前相同 |
| **Data** | 所有文件 | 拆分归档：`<name>.mpxar.didx`（元数据）+ `<name>.ppxar.didx`（文件内容） |
| **Metadata** | 仅新增和修改的文件 | 同样的拆分归档；大小、修改时间（精确到纳秒）、权限模式和属主与上次备份一致的文件直接从上次备份中取用，不读取 |

在 **Metadata** 模式下，首次运行读取全部内容（还没有可比较的对象）；之后的运行只读取变化的部分。如果文件的修改没有改变其大小和修改时间，就不会被发现，因此备份集也可以设置**每 N 次运行读取所有文件**（一次 Data 运行）：这次运行会捕捉到这类修改，并去掉复用在归档中留下的填充。一个未能完整读取的文件（读取出错后以零填充）在下次运行时总会被重新读取。只有加密方式和密钥都相同的上一次备份才会被复用。

由于未更改的数据既不读取也不发送，到远程或异地 PBS 的流量也会减少到几乎只剩实际变化的部分。

**GUI：** 编辑 Backup Set，**Destination** 标签页，**Change detection**。它只适用于文件夹备份集：整机（整盘）备份是磁盘镜像，不受影响，因此裸机恢复和 Proxmox VE 恢复与以前完全一样。一次性备份使用 Legacy。只有成功完成的运行才计入下一次完整读取；Backup Sets 列表显示“Next full read in N runs”。进度卡片会说明某次运行何时以及为何读取所有文件。**Reports** 显示每次运行的模式以及复用了多少个未更改的文件。

**每个备份集一个 Backup ID。** Metadata 模式与备份组中最新的快照比较。如果两个备份集（或两台机器）在同一服务器上使用相同的 Backup ID，每次运行都会与对方的快照比较，从而读取全部内容。Backup Set 编辑器会对此发出警告。

**命令行：** `-change-detection-mode legacy|data|metadata`，或 JSON 配置中的 `"change-detection-mode"`。命令行没有“每 N 次运行”的参数：请改为偶尔安排一次 `-change-detection-mode data` 运行。

**在把备份集切换到 Data 或 Metadata 之前，** 请把所有需要从中恢复的机器升级到 v0.6.0 或更高版本：支持拆分归档之前的版本无法读取拆分归档（只显示文件夹，不显示其中的文件）。官方 `proxmox-backup-client` 能逐字节一致地恢复它们（已用 3.4.9 测试），PBS 4.2 的文件浏览器也能打开它们。一个文件夹的第一次拆分备份与其旧的 Legacy 备份共享的数据很少，因此在旧快照被清理之前，它大约会在数据存储上多占用该文件夹一份完整副本的空间。

## 🚀 快速开始 (GUI)

1. 安装：Windows 用 MSI，Linux 用 `.deb` 或 `.rpm`，或从[发布页](https://github.com/mjb-is/proxmoxbackupclient_go/releases)解压 zip 或 tar.gz（见[下载和安装](#-下载和安装)）
2. 启动。Windows 上，VSS、整机备份和修改 Backup Sets 需要 **Run as administrator**（以管理员身份运行）。Linux 上，备份你自己的文件以普通用户身份即可；整机备份和块快照请使用 **Proxmox Backup Client (administrator)**（或以 root 运行）
3. 在 **Preferences > Account Information** 中添加你的 PBS 服务器，并点 **Test** 测试
4. 创建一个 Backup Set（或一次性备份），选择要备份的文件夹或磁盘
5. 运行。要让计划在无人登录时也能运行，请开启 **Preferences > Advanced > Run as Service**

[GUI 手册](docs/manual/GUI.md)逐步讲解每个步骤。

## 整机在线备份

`proxmoxbackup-machine`（以及 GUI 的整机备份）可在不停机的情况下备份完整的 Windows 10/11 或 Windows Server 系统，或一台 Linux 机器。

命令语法与文件夹备份基本相同，只是用 `-backupdev` 代替 `-backupdir`（有多块磁盘时重复该参数）。
例如可以这样调用：

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

上面的命令会检查磁盘 0，找出所有已挂载的分区，对它们创建 VSS 快照，然后把整盘生成一个可启动的 fidx 备份镜像。空闲空间、分页文件和休眠文件不读取，以零写入镜像（在 Linux 上，ext2/3/4 和 XFS 的空闲空间也一样）；`-readfreespace` 则读取全部内容，用于取证副本。无法读取的扇区以零写入镜像，运行结果会列出其位置。加上 `-type vm` 可把备份存为 Proxmox VE VM 备份（[P2V](#-物理机转为虚拟机-p2v)）。

下一次备份是增量的；哈希计算已并行化，可以轻松达到 1 GB/秒的速度。

### Linux 整机备份前提条件（VSS 的对应物）

Windows 整机备份使用 VSS（所以上面需要“以管理员身份运行”）；Linux 没有 VSS，因此本项目使用 `elastio-snap` 内核模块（`dattobd` 的 fork）完成同样的工作：对一个仍在被写入的在线块设备做一致的时间点快照。原 `elastio/elastio-snap` 仓库已归档；维护中的版本是 [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)，请安装这个。如果 `dattobd` 已经加载，也同样支持。

以下两条都是必需的，相互独立，缺一则 Linux 整机备份会拒绝运行：

1. **必须安装并加载内核模块。** `elastio-snap` 文档中通过仓库软件包安装的方式（见[其 INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)）目前至少在 Ubuntu 22.04 上指向一个已迁移或失效的 URL（2026-09-24 确认：仓库软件包在重定向后返回 404）。从源码构建则很可靠：
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
   这只为本次启动构建并加载模块；如果需要每次开机自动加载，请参考你的发行版文档（例如 `/etc/modules-load.d/` 中的条目）。

2. **备份进程本身必须以 root (EUID 0) 运行。** 即使模块已加载，`CreateVSSSnapshot`（尽管名字偏向 Windows，它就是 Linux 的快照路径）也会明确拒绝以非 root 用户运行：一致的整盘快照需要模块中仅限 root 的控制接口，所以这是硬性要求，加入 `disk` 组之类的方式无法替代。整机备份请使用 **Proxmox Backup Client (administrator)** 菜单项、后台服务，或用 `sudo`（或以 root）运行 GUI 或 `proxmoxbackup-machine`；普通的文件/目录备份没有这项要求。

如果整机备份报错 `backup device /dev/sdX: open /dev/sdX: permission denied`，通常只是需要把你的用户加入 `disk` 组以获得基本的读取权限（`sudo usermod -aG disk $USER`，然后注销并重新登录），但仅此一项并**不足以**做真正的快照；紧接着你仍会遇到上面的 root 要求。

### 从磁盘备份中恢复文件

GUI 的 Restore 页面像打开文件夹备份一样打开整机备份：镜像中的每个卷（NTFS、FAT、exFAT、ext2/3/4 或 XFS）都是一个可以浏览和恢复的文件夹。不需要挂载任何东西。

在 Linux 上，也可以用 nbd 工具把磁盘备份挂接为块设备。

要使用 nbd，请先执行 `modprobe nbd max_part=0`。

原因不明，使用 `max_part != 0` 会导致无限的分区探测循环。

NBD 工具可以连接任何固定磁盘备份，无论是 VM 备份还是 host 备份（也就是说，它同样适用于 PVE 备份）。

使用时，命令类似：
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

如果省略 `-path`，会出现一个终端界面供你选择 fidx 文件。

> 不要在运行重要服务的机器上使用它：损坏的文件系统可能导致操作系统崩溃，这也是 Proxmox VE 为此使用 QEMU 实例的原因。

停止 `proxmoxbackup-nbd` 之前，请卸载 nbd 磁盘上的所有内容，否则该分区很可能一直处于忙碌状态，无法卸载。

如果出现 `Device or resource busy` 错误，需要运行 `nbd-client -d /dev/nbd0` 强制断开，或直接重启。

### 恢复到物理机或虚拟机

裸机恢复 ISO 带有全自动的 **“PBS Bare Metal Restore”** 启动菜单项：见下文[裸机恢复 ISO](#-裸机恢复-iso)。它位于首位，超时后默认启动，并跳过 Clonezilla 自身的语言和键盘提示：输入 PBS 连接信息，选择快照，确认自动检测到的目标磁盘，之后网络设置、NBD 挂接、恢复本身和自动重启都无需再操作。同一个 ISO 也能恢复到空虚拟机中，用于 [P2V](#-物理机转为虚拟机-p2v)。

较早的半手动方式也仍在同一 ISO 上，作为 Clonezilla 常规模式选择菜单中独立的一项（“pbs-nbd”）：它引导你输入 PBS 连接信息，在后台把所选快照挂接到 `/dev/nbd0`，然后由你通过 Clonezilla 自带的标准磁盘恢复向导手动完成恢复。

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

官方客户端的 `PBS_REPOSITORY`、`PBS_SERVER`、`PBS_PORT`、`PBS_DATASTORE`、`PBS_AUTH_ID`、`PBS_PASSWORD` 和 `PBS_FINGERPRINT` 环境变量也会被读取。

JSON 配置提供了一个 JSON 示例，只需填写需要的字段。

关于邮件模板：
邮件主题和正文使用 [Go 的模板引擎](https://pkg.go.dev/text/template)，语法请参阅其文档。
模板中可用的变量如下：

- `.NewChunks`：新建的块数
- `.ReusedChunks`：复用的块数
- `.Datastore`：数据存储名称
- `.Error`：错误（如有）（`.ErrorStr` 以文本形式给出，没有错误时为空）
- `.ReadErrors`：无法读取的文件或文件夹；`.ReadErrorCount` 是其数量
- `.Hostname`：机器的主机名
- `.StartTime`：备份开始时间
- `.EndTime`：备份结束时间
- `.Duration`：备份耗时
- `.FromattedDuration`：格式化后的备份耗时（代码中就是这样拼写的）
- `.Success`：备份完成、没有错误且没有无法读取的内容时为 true
- `.Partial`：快照已保存但有部分文件无法读取时为 true
- `.Status`：`Success`、`Partial` 或 `Failed`

## 流备份

这样可以备份数据流而不是 pxar，带来无限可能。例如你可以这样调用：

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

这样即使使用 tar 之类的工具，或直接备份 SQL 转储本身，也能利用 buzhash 进行去重。

## 已知问题

启用 Windows Defender 反恶意软件会使备份速度最多降低 25%。

在 Windows 上，命令行工具会获取一个进程锁：在一个备份运行期间启动第二个备份，会打印 “Another backup job is running” 并退出。Linux 上没有这样的锁，所以不要把两个命令行运行安排在同一时间。

## 📋 环境要求

- Windows 10/11 或 Windows Server（64 位），VSS 和整机备份需要管理员权限
- 或 Linux（软件包适用于 Debian、Ubuntu、Mint 和 Fedora），GUI 需要 WebKitGTK 运行库，一致的整机备份需要 root 以及 elastio-snap 或 dattobd 内核模块
- 能访问 Proxmox Backup Server 的网络
- 用于 P2V：已把 PBS 数据存储添加为存储的 Proxmox VE，或任何能启动恢复 ISO 的虚拟化平台

## 🔨 从源码构建

**[BUILD.md](BUILD.md)** 有完整说明：GUI 和后台服务、命令行工具、MSI 以及 Linux 软件包。简要来说，需要 Go 1.25 或更高版本和 Wails CLI v2.13：

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

```bash
packaging/debian/build-deb.sh     # .deb
packaging/fedora/build-rpm.sh     # .rpm (in Docker)
```

```powershell
installer\wix\build-msi.ps1       # MSI (WiX 3), after building the app and the service
```

已构建的前端 (`gui/frontend/dist`) 已提交到仓库，所以只有修改前端时才需要 Node.js（在 `gui/frontend` 中运行 `npm install` 和 `npm run build`，或用 `wails dev` 热重载）。测试：`make test`。

## 🔧 高级用法与指南

### 多 PBS（多台 PBS 服务器）

配置多台 PBS 服务器，为每个 Backup Set 或一次性备份选择目标（例如 `C:\Users` 每天备份到快速 SSD 的 PBS，`D:\` 每周备份到大容量 PBS，另加一台灾备服务器）。

- **[用户指南](MULTI_PBS_USER_GUIDE.md)**：添加和测试服务器、默认服务器、为每个备份选择服务器、常见问题和故障排除。
- **[实现指南](MULTI_PBS_GUIDE.md)**：数据模型、从单 PBS 配置自动迁移、后端方法。

旧的单 PBS 配置在首次加载时会自动迁移为 `default` 服务器。

### 💿 裸机恢复 ISO

恢复 ISO 是一个标准 Clonezilla Live ISO，打了补丁加入 `pbsnbd` / `machinebackup` 二进制文件和两个菜单项。它可以从光盘、U 盘（用 `dd` 写入，或使用 Ventoy U 盘）、PXE 启动，支持 UEFI 和 BIOS 机器：

- **`ocs-pbs-bare-metal-restore`**：位于首位、超时后默认启动的启动菜单项 **“Proxmox Backup Client Go - PBS Bare Metal Restore”**。它跳过 Clonezilla 的语言和键盘提示以及通用向导，从输入 PBS 凭据直接走到恢复完成并自动重启：先尝试 DHCP（失败时才提示静态 IP）、NTP 同步、备份加密时从 U 盘读取加密密钥，以及从不把启动用 U 盘列为目标的目标磁盘检测。成功后显示完成信息并重启。Windows 和 Linux 备份同样适用（恢复在块级别进行），可恢复到裸机或空虚拟机中。
- **`ocs-pbs-nbd`**：Clonezilla 自身模式选择菜单中的 **“pbs-nbd”** 项。提示输入 PBS 连接信息和要恢复的快照，在后台把它挂接到 `/dev/nbd0`，然后交给 Clonezilla 的常规磁盘恢复向导，由你手动操作。

恢复方面还修复了：PBS 取块路径中导致整机恢复挂起的 `zstd.NewReader` 死锁（已作为 [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85) 贡献给原项目）；由 Clonezilla 自身的 `is_disk_without_part_and_fs()` 与 udev 竞争导致的“第一次恢复因 'no partition' 失败、重跑成功”的错误；以及 BIOS/MBR 机器恢复后无法启动的问题（现在 MBR 与第一个分区之间的数据也会被恢复）。

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

完整细节（为什么要完整重建 ISO 而不是原地替换、前提条件、菜单流程、验证）见 **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**；使用方法见 **[裸机恢复手册](docs/manual/BMR.md)**。

### 构建 Windows GUI

**原生 Windows**：工具链（Go、MinGW、Wails）以及应用、服务和 MSI 的构建命令见 **[BUILD.md](BUILD.md)**。

**Docker（在 Linux 上构建时）。** `./build_gui_windows_docker.sh` 在一次性的 `golang` 容器中把 Windows 应用构建为 `ProxmoxBackupClientGO.exe`，带完整的 WebView2 支持（安装 mingw 和 Wails，构建前端，运行 `wails build`）。它不构建后台服务和 MSI。

### 功能状态、更新日志和内部文档

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**：已完成、部分完成或未完成的功能，以及从哪个版本开始。
- **[CHANGELOG.md](CHANGELOG.md)**：按版本的变更历史，[Unreleased] 为 master 上的内容。
- **[TODO.md](TODO.md)**：开放的路线图和想法。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**：每个版本包含的内容。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**：MSI 卸载对话框（保留/删除配置）及其测试计划。
- **[AGENTS.md](AGENTS.md)**：仓库结构和约定，供贡献者和编码代理参考。

## 🔁 对原项目的贡献

具有通用价值的修复和功能会以 pull request 的形式提交给 [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go)。已有十八个被合并：

  - [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85): Fix zstd decoder goroutine deadlock in `GetChunkData` (hung full-machine restores)
  - [PR #86](https://github.com/tizbac/proxmoxbackupclient_go/pull/86): Fully automated PBS Bare Metal Restore boot entry for the Clonezilla ISO
  - [PR #87](https://github.com/tizbac/proxmoxbackupclient_go/pull/87): Linux snapshots: drive elastio-snap/dattobd through ioctl instead of `elioctl`/`dbdctl`
  - [PR #88](https://github.com/tizbac/proxmoxbackupclient_go/pull/88): Restore: re-apply captured NTFS ACLs (Windows) and POSIX ACLs and xattrs (Linux)
  - [PR #90](https://github.com/tizbac/proxmoxbackupclient_go/pull/90): `machinebackuplib`: make `vm` backups restorable and bootable on Proxmox VE
  - [PR #91](https://github.com/tizbac/proxmoxbackupclient_go/pull/91): GUI: back machines up as `host`, not `vm`, by default
  - [PR #92](https://github.com/tizbac/proxmoxbackupclient_go/pull/92): `patch-clonezilla`: skip patches whose change is already in the base ISO
  - [PR #93](https://github.com/tizbac/proxmoxbackupclient_go/pull/93): `patch-clonezilla`: add the PBS bare-metal restore entry to `isolinux.cfg`
  - [PR #94](https://github.com/tizbac/proxmoxbackupclient_go/pull/94): Restore the data between the MBR and the first partition in the BMR flow (BIOS/MBR machines boot after restore)
  - [PR #95](https://github.com/tizbac/proxmoxbackupclient_go/pull/95): Fix reading passphrase-protected key files made by `proxmox-backup-client`
  - [PR #96](https://github.com/tizbac/proxmoxbackupclient_go/pull/96): `directorybackup`: back up several directories in one run
  - [PR #97](https://github.com/tizbac/proxmoxbackupclient_go/pull/97): Docs: point elastio-snap at the maintained Axcient fork
  - [PR #98](https://github.com/tizbac/proxmoxbackupclient_go/pull/98): `pbscommon`: skip the key file permission check on Windows
  - [PR #99](https://github.com/tizbac/proxmoxbackupclient_go/pull/99): Restore encrypted backups from the Clonezilla BMR ISO
  - [PR #102](https://github.com/tizbac/proxmoxbackupclient_go/pull/102): `snapshot`: delete our VSS shadow copies after every backup, through the VSS API
  - [PR #103](https://github.com/tizbac/proxmoxbackupclient_go/pull/103): `machinebackup`: skip hashing blank (all-zero) chunks
  - [PR #104](https://github.com/tizbac/proxmoxbackupclient_go/pull/104): CLIs: report errors on stderr, never in a message box
  - [PR #105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105): Restore: put back every file time, also 1970 and earlier, and folder times

## 🧪 状态、错误报告和功能请求

**这是 alpha 软件，按原样提供，不提供任何形式的保证或担保，使用风险完全由你自己承担。** 对于任何数据丢失或损坏（包括源文件），我不承担任何责任。

它是为我自己的家庭实验室开发的。每个版本发布前都经过我亲自测试，并且在那里日常使用：对 Windows 和 Linux 机器进行计划的文件夹备份和整机备份，期间也测试了恢复、Undelete、Roll back、裸机恢复和 P2V 恢复。任何备份在你真正从中恢复之前都不算得到验证，所以在依赖它之前，请先测试你自己的恢复，包括一次裸机恢复。

- **欢迎报告错误**：请提交 [issue](https://github.com/mjb-is/proxmoxbackupclient_go/issues)，写明你做了什么、发生了什么，并附上日志（Message Log，或设置目录中的日志文件）。
- **功能请求**：我只添加对我自己的使用有帮助的功能，所以不接受为他人定制的功能请求。请把它们发送到原项目 [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues)。欢迎修复错误的 pull request，或明显也对我的使用有帮助的 pull request。

## 🙏 致谢与历史

最初的命令行客户端由 Tiziano Bacocco (tizbac) 自 2023 年起编写。后来 RDEM Systems 的图形客户端被合并进来，随之带来了第一个 GUI、多 PBS 支持和 Windows 服务。本 fork 始于 2026 年 9 月，用来填补那些让 PBS 无法成为我唯一备份系统的空缺；它的版本从 v0.3.0 开始，具有通用价值的修复会回馈给原项目。

- 原始客户端 [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go)，作者 Tiziano Bacocco (tizbac) 及其贡献者。
- Backup for Workgroups (BFW) 的工作流程和使用感受，Backup Sets、Undelete 和 Roll back 都沿用了它。
- [Clonezilla Live](https://clonezilla.org/)，裸机恢复 ISO 以它为基础构建。

## 📄 许可证

GPLv3：见 [LICENSE](LICENSE) 文件。

---

**fork 之后的改动 © 2026 mjb-is，基于 Tiziano Bacocco (tizbac)、Proxmox Backup Client GO 贡献者和 RDEM Systems 的工作 (© 2024-2026)。**
