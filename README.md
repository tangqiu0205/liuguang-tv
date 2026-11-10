# 流光 TV（LiuguangTV）

基于 Go 和 [MyGo](https://github.com/egoist/mygo) 的 IPTV 桌面客户端。频道、设置与控制使用原生 UI，支持 **mpv**（默认）和 **内置 WebView** 两种播放引擎。

本应用不预置节目源，也不捆绑或自动下载 mpv。请仅使用拥有观看权限的节目源。

## 下载与运行

从 [GitHub Releases](https://github.com/tangqiu0205/liuguang-tv/releases) 下载对应平台和架构的包。Windows 提供安装程序和便携 ZIP；macOS 提供 DMG 和原生应用归档；Linux 提供 DEB 和原生 tar.gz。打包结构与 `tiaopin-fm` 一致，但不附带 mpv。

| 平台 | 发布架构 | 说明 |
| --- | --- | --- |
| Windows | amd64 / arm64 | 安装 `liuguangtv-setup-VERSION-ARCH.exe`；或解压 `liuguangtv-portable-VERSION-windows-ARCH.zip` 后运行根目录的 `LiuguangTV.exe` |
| macOS | amd64 / arm64 | 安装 DMG 中的 `LiuguangTV.app`，或从 `liuguangtv-VERSION-darwin-ARCH.tar.gz` 解压应用包 |
| Linux | amd64 / arm64 | 安装 DEB，或解压 `liuguangtv-VERSION-linux-ARCH.tar.gz` 后运行根目录的 `liuguangtv`；需要 GTK 3、GLib、Cairo、GdkPixbuf、OpenGL/EGL |

Windows 便携版以根目录的 `portable.flag` 启用便携模式，配置、缓存和日志保存在 exe 同目录的 `data/` 中；移动时复制整个目录，不要删除标记。安装版及新 macOS/Linux 原生归档使用系统用户配置目录，不因解压到某个目录就自动启用便携模式。

旧便携版的 `app/`、同级 `data/` 和 `app/liuguangtv-portable.mode` 布局继续受支持，应用内更新后仍读取原数据。手动换用新版 Windows ZIP 时，先退出并备份，再将旧 `data/` 复制到新 exe 同目录；macOS/Linux 手动换用原生包时可用设置中的导入导出迁移。新旧便携标记不要混用，标记丢失但存在便携数据时会拒绝静默切换配置目录。

发布包附有 `SHA256SUMS.txt`。Windows 程序未做代码签名，macOS 包仅做临时签名、未经公证；请确认来源并核对校验值，不要关闭系统安全保护。Linux 发布构建使用 Ubuntu 24.04，不保证兼容更旧系统或所有发行版。

1. 安装 [mpv](https://mpv.io/installation/)，或在设置 → 播放器中选择内置 WebView。
2. 默认自动查找 mpv；也可在设置 → 播放器中点击“下载 mpv”打开官方安装与下载页面，自行下载并安装或解压，再浏览选择可信的 mpv 可执行文件完整路径。应用不会自动下载或执行安装程序。
3. 在设置 → 订阅源 → 添加订阅源中选择类型：“频道订阅”支持 HTTP(S) 或本地 M3U / M3U8 / TXT；“EPG 节目单”支持 XMLTV / XMLTV.gz。手动节目单显示在 EPG 页，订阅自带的节目单统计显示在订阅卡片中。
4. 返回播放页，点击频道播放；星标按钮用于收藏。

[`examples/channels.m3u`](examples/channels.m3u) 仅展示格式，其中地址不可播放。不要将 HLS 分片清单直接当作频道列表导入。

## 功能

- 多订阅管理、排序、启停、单独刷新及每 24 小时后台更新，失败保留缓存。
- 按订阅、分组、名称和收藏筛选频道，支持虚拟滚动。
- 合并同频道多线路，手动切线；支持的引擎具备有限次数的故障切线。
- 显示 M3U 中 `tvg-logo` 指定的频道台标，无法加载时显示默认图标。
- Windows 内嵌 mpv 播放，暂停、停止、静音、音量，以及窗口全屏和系统全屏。
- 可选 WebView 播放，媒体脚本离线内嵌，原生界面负责控制。
- XMLTV / gzip EPG 节目单，支持多日期浏览、当前 / 下一个节目和本地缓存。
- 跟随系统、日间和夜间主题。
- 简体中文与英文界面；设置 → 外观 → 界面语言中即时切换，自动保存，也可恢复跟随系统。
- 配置导入导出、损坏恢复和有大小上限的脱敏本地诊断日志。

## 限制

- 没有录制或节目回看功能。签名发布版本可检查应用更新；开发构建不支持应用内更新。
- 非 Windows 的 mpv 使用独立窗口，应用内暂停 / 音量与 IPC 断流检测不可用；窗口定位仅为启动时尝试，不会持续跟随主窗口。
- WebView 仅支持无 URL 用户凭据的 HTTP(S) 流，编解码能力取决于系统，不保证 H.265 / AC-3 等可用。Windows 需要 WebView2 Runtime，Linux 还需要 WebKitGTK。
- macOS / Linux 的真实桌面播放、长时播放、休眠唤醒和多屏 DPI 行为仍需人工验证。自动化测试不能替代这些验证。

## 从源码运行

需要 Go **1.27.3 或更高版本**，以 [go.mod](go.mod) 为准。在仓库根目录执行：

```bash
go run ./cmd/liuguangtv
go test ./...
go vet ./...
```

Windows 一键检查并构建：

```powershell
./build.ps1
```

输出为 `dist/LiuguangTV.exe`。MyGo 打包工具由 `go.mod` 的 `tool` 声明固定，可用 `go tool mygo dev` 开发、`go tool mygo build` 打包，无需单独安装 CLI；生成签名更新仍需配置发布密钥。Go 构建不需要 Node、Bun 或 C 编译器；运行原生窗口仍需要图形桌面及对应系统运行库。发布脚本另需 Python 等工具，详见开发与发布文档。

## 应用内更新

设置 → 关于 → 检查更新，会打开 MyGo 原生更新窗口，显示更新说明、下载进度和重启选项，语言跟随应用设置。

- 正式构建默认启动约 10 秒后检查，此后每天检查一次；自动下载默认关闭，可在更新窗口中开启。
- 更新从 GitHub Releases 获取，并使用应用内置公钥验证签名。支持 MyGo 增量更新；没有合适的增量包时使用全量包。检查或下载失败会显示错误，不会跳过验签。
- 可写目录中的 Windows、macOS 和 Linux 应用支持自更新；便携版保留数据目录与便携标记。Linux DEB 等包管理器安装方式需通过包管理器或重新下载安装包更新。
- `go run` / 普通 `go build` 开发构建未启用更新；手动检查会说明原因。更新包签名与 Windows/macOS 的系统代码签名是两回事。

发布需配置与 `mygo.json` 公钥匹配的 Actions Secret `MYGO_UPDATER_PRIVATE_KEY`，并准备对应版本的更新日志；不要更换现有公钥，否则已安装版本无法验证后续更新。流程见 [构建与发布](docs/RELEASING.md)。

## 界面语言

首次启动默认跟随系统语言，不支持的语言回退为简体中文。设置 → 外观中的语言选项与主题一致：`跟随系统`、`简体中文`、`English`。高亮表示保存的选择，下方“当前语言”显示实际生效的语言。语言选择由 MyGo 持久化；Windows/Linux 保存在用户数据目录的 `locale` 文件中，macOS 使用应用的系统语言偏好，不写入 JSON 配置备份。

切换语言不停止播放，不修改频道、分组、订阅或节目的原始名称。已生成的状态消息、缓存的错误详情与底层服务错误保留生成时的原文；后续消息使用新语言。

翻译维护约定见 [国际化开发说明](docs/I18N.md)。

## 文档

- [使用指南](docs/USAGE.md)：订阅、播放、快捷键、EPG 与请求头。
- [配置备份与恢复](docs/BACKUP.md)。
- [隐私与数据](docs/PRIVACY.md)。
- [WebView 实现与依赖](docs/WEBVIEW.md)。
- [更新日志](CHANGELOG.md)。
- [构建与发布](docs/RELEASING.md)：版本、签名与 CI。
- [第三方许可说明](THIRD_PARTY_NOTICES.md)。

## 许可与反馈

**仓库尚未声明项目代码的开源许可证。** 公开可见不等于已授予使用、修改和再分发许可；正式开源前需由权利人选择许可证并补充 `LICENSE`。第三方组件继续受各自许可证约束。

问题反馈请前往 [Issues](https://github.com/tangqiu0205/liuguang-tv/issues)，提供系统、架构、版本和复现步骤。上传日志、截图或配置前，请删除个人信息、订阅账号和 URL 令牌。
