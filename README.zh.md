# Proxmox Backup Client — Proxmox Backup Server 的 Windows 客户端

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client 是一个面向 Proxmox Backup Server (PBS) 的开源 (GPL-3.0) 备份客户端，支持 Windows 和 Linux。**
它是一套用于备份到 PBS 的**工具套件**：

- **Proxmox Backup Client GUI**（基于 RDEM Systems 的 Nimbus Backup GUI）——现代化的图形界面，用于将 Windows 服务器和工作站备份到 PBS：一致的 VSS 快照、计划任务、文件和磁盘模式、快照浏览/恢复、多 PBS 支持以及 Windows 服务模式。
- **`proxmoxbackup-directory`** —— 用于目录 (PXAR) 备份（含去重）的命令行工具。
- **`proxmoxbackup-machine`** —— 用于完整在线备份 Windows 系统 (FIDX、VSS、增量) 的命令行工具。
- **`proxmoxbackup-nbd`** —— 用于恢复磁盘备份的 NBD 服务器 (Linux)。

> 关键词：proxmox 备份客户端 windows · PBS 客户端 · Windows VSS 备份 · 不可变异地备份 · Proxmox Backup Server 接口。

> ⚠️ **免责声明：** 本项目与 **Proxmox Server Solutions GmbH** **没有任何关联**。「Proxmox」、Proxmox 徽标及相关名称归其各自所有者所有；此处使用它们**仅**用于表明兼容性。请访问 [proxmox.com](https://www.proxmox.com/) 了解他们的产品。

> 🤖 **本翻译由 AI 生成，可能包含一些小的错误。欢迎您贡献改进。**

## 📦 下载

👉 **[下载最新版本](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **Windows 显示“检测到病毒”（例如 `Trojan:Win32/Sabsik.FL.A!ml`）或 SmartScreen 警告？**
> 这是 Go/Wails 应用程序的**已知误报**——它*不是*病毒。`!ml` 后缀表示机器学习模型检测，会标记*未签名且不常见*的可执行文件。
> 请参阅[为什么会出现这种情况以及如何验证下载](https://github.com/tizbac/proxmoxbackupclient_go)。

### 🔎 验证任何下载

每个版本都提供 SHA-256 校验和和**带签名的来源证明**（密码学证明该二进制文件是由本仓库的 CI 从某个精确提交构建的）：

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # 与 SHA256SUMS.txt 比较
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 项检测。** 近期 MSI 安装程序的独立多引擎报告：
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **代码签名：** Windows 二进制文件**尚未进行 Authenticode 签名**（通过 [SignPath Foundation](https://signpath.org) 申请的 OSS 证书待审核）。在此之前，来源通过上述证明和校验和来确认。

## 📚 文档

- **Proxmox 备份完整指南** — PBS 部署最佳实践
- **使用 Proxmox Backup Server 备份 Windows** — 特定 Windows 部署指南
- **PBS 与 Veeam 对比** — Proxmox 备份对比

## ✨ 功能

### Proxmox Backup Client GUI（推荐）
- **🌍 多语言** — 界面支持中文、英文等多种语言
- 友好的配置界面，带连接测试
- 实时备份进度，显示速度与剩余时间
- VSS（卷影副本）支持，保证备份一致性
- 多文件夹备份、文件和磁盘模式
- 快照浏览、文件搜索（通配符）和恢复
- 多 PBS 服务器支持，证书指纹固定（TOFU）
- Windows 服务模式 + 计划备份
- 调试日志记录，便于诊断

### 📸 截图

![服务器配置](docs/screenshots/nimbus-gui-liste-servers.png)
*带状态指示器的多 PBS 服务器管理*

![添加服务器表单](docs/screenshots/nimbus-gui-add-server-form.png)
*带连接测试的简单服务器配置*

![即时备份](docs/screenshots/nimbus-gui-one-shot-backup.png)
*带 ETA 和吞吐量的实时备份进度*

### 智能系统排除（文件模式）
备份整个驱动器（例如 `D:\`）时，Proxmox Backup Client 会自动排除：

**系统文件夹：** `System Volume Information`（VSS 存储，可达 100+ GB）、`$RECYCLE.BIN`、`Recovery`。
**系统文件：** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys`。

**为什么重要：** 一个驱动器可能显示已用 1.03 TB，而真实文件只有约 141 GB。不做排除的话，备份会包含 VSS 快照（浪费空间和时间）；做了排除，大小就会与真实数据相符。

**建议：** 文件级备份使用**文件模式**（默认）并启用自动排除；裸机恢复（包含一切）请在单独任务中使用**磁盘模式**。

### 安全与质量
- 输入验证和凭据清理
- 路径遍历防护
- 指数退避重试逻辑
- 全面的错误处理和测试，100% 符合 lint

## 🚀 快速开始

1. 从发版页下载 `ProxmoxBackupClient.exe`（或 `.msi`）
2. 以管理员权限运行（VSS 需要）
3. 配置 PBS 连接并测试
4. 选择要备份的文件夹
5. 开始备份

## 📋 环境要求

- Windows 10/11（64 位）
- 管理员权限（用于 VSS 快照）
- 可访问 Proxmox Backup Server 的网络连接

## 🔨 从源码构建

### 前置要求
- Go 1.22 或更高版本
- Node.js 20 或更高版本
- Wails CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### 构建
```bash
cd gui
npm install --prefix frontend
wails build      # 或：wails dev（热重载）
```

## 🔧 高级用法与指南

### 多 PBS（多个 PBS 服务器）

配置多台 PBS 服务器，并为每次备份选择目标（例如 `C:\Users` → 快速 SSD PBS，每日；`C:\` → 大数据 PBS，每周；外加一个灾备服务器）。

- **[用户指南](MULTI_PBS_USER_GUIDE.md)** — 添加/测试服务器、默认服务器、常见问题与故障排除。
- **[实现指南](MULTI_PBS_GUIDE.md)** — 数据模型、从单 PBS 配置的自动迁移、后端 API 方法。

现有的单 PBS 配置会在首次加载时自动迁移到 `default` 服务器。

### Clonezilla ISO（裸机恢复）

救援流程是通过修补 Clonezilla Live ISO，加入 `pbsnbd` / `machinebackup` 二进制文件，并在 Clonezilla 主菜单中添加 **pbs-nbd** 条目（支持 CD、USB 通过 `dd` 及 UEFI 启动）：

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
```

完整细节（为什么全面重建 ISO 而不是原地替换、前置要求、菜单流程、验证）请参见 **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**。

### 构建 Windows GUI

**Docker（推荐，尤其在 Linux 下构建）。** 一键脚本会生成一个带正确 WebView2 支持的 `ProxmoxBackupClientGO.exe`，使用一次性 `golang` 容器（安装 mingw + Wails、构建前端、执行 `wails build`）：

```bash
./build_gui_windows_docker.sh
```

**原生 Windows（Chocolatey）。** 完整的 Windows 工具链设置请见 **[BUILD.md](BUILD.md)**：

```powershell
choco install go
choco install mingw
# 然后，在非提权 shell 中：
build.bat          # GUI
build_cli.bat      # CLI
```

### 功能状态、更新日志和内部文档

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** — 各功能状态矩阵（已实现 / 已测试 / 路线图）。
- **[CHANGELOG.md](CHANGELOG.md)** — 按版本记录变更历史。
- **[TODO.md](TODO.md)** — 开放的路线图和想法。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** — 稳定产品状态和可用构建。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** — MSI 卸载对话框（保留/删除配置）及其测试计划。
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** — GUI 修复说明（目录模式与机器模式切换）。

## 🖥️ GUI 归属

**Proxmox Backup Client GUI** 基于 **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**，由 **[RDEM Systems](https://www.rdem-systems.com/)** 开发和维护。

该 GUI（最初是本项目的 fork）已合并回本仓库：包括 GUI 及其所有功能在内的全部代码继续保持 GPLv3 开源。RDEM Systems 赞助 GUI 开发并为其提供商业支持。

**原始 CLI 作者：** Tiziano Bacocco (tizbac) · **许可证：** GPLv3

## ⚠️ 警告

本软件按“原样”提供。尽管我们追求可靠性，但对数据丢失或损坏不承担任何责任。在将备份用于生产环境之前，请务必测试备份并验证恢复。

## 📄 许可证

GPLv3 — 参见 [LICENSE](LICENSE) 文件。

## 🏷️ 品牌

凡贡献了**至少 5 个 commit**、增加功能或修复的贡献者，都有权将自己的品牌数据用于商业用途。

唯一条件是品牌指向的公司**没有**从事以下任何活动：

- 恶意软件（malware）宣传活动
- 宣扬战争的企业（适用于任何国家，包括西方国家）
- 欺诈
- 窃取数据
- 贩卖人口/儿童
- 暴力
- 歧视
- 毒品
- 任何被普遍认为非法的活动

如果针对任何贡献者出现投诉，我们会尝试联系；如果没有给出合理解释，我们将**立即终止**该权益。

**GPLv3 许可证仍然有效**，您仍然可以自由 fork 本项目并构建自己的可执行文件。

## 关于 Proxmox Backup Client GO 的贡献者

Proxmox Backup Client GO 的贡献者开发并维护本项目。该软件依赖 NTP/NTS 基础设施，以及社区参考中列出的 [11 个公共 NTS 服务器](https://github.com/jauderho/nts-servers)。

## 🤝 贡献

GUI 现已完全实现，但仍欢迎贡献，尤其是：

1. 加密支持（仍然缺失）
2. 物理机到虚拟机 (P2V) 迁移，将裸机备份恢复到虚拟机（仍不完整）
3. 异步上传 / 多核上传 chunk（多核压缩已为 machine backup 实现）
4. Proxmox 端补丁，向 pxar 格式添加一种带有 Windows 安全描述符的新条目
5. Windows 符号链接支持
6. 任何您觉得有趣的想法 :)

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**