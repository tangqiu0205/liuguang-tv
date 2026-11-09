# 构建与发布

## 版本与构建工具

正式发布的唯一版本输入是 `vMAJOR.MINOR.PATCH` 标签。CI 校验标签与检出提交，构建时将版本写入临时工作区的 `mygo.json`，并通过 `MYGO_LDFLAGS` 注入程序版本；启动时两个版本不一致则拒绝运行。普通 `go build` 默认为 `dev`，`build.ps1 -Version` 可显式指定版本。

MyGo CLI 通过 `go.mod` 的 `tool github.com/egoist/mygo/cmd/mygo` 声明，使用 `go tool mygo build`，与应用共用模块版本。仓库中的 `mygo.json` 版本只是本地构建配置，CI 不提交其临时版本变更。

## 平台与产物

构建矩阵与 `tiaopin-fm` 一致：

- macOS：`macos-15` 构建 arm64，`macos-15-intel` 构建 amd64。
- Windows：在 `ubuntu-24.04` 安装 NSIS，一次交叉构建 amd64 和 arm64；另设 `windows-latest` 任务运行 Windows 测试和 vet。
- Linux：在 `ubuntu-24.04` / `ubuntu-24.04-arm` 分别构建 amd64 / arm64。更旧系统的兼容性需另行验证。

| 平台 | 安装包与分发包 |
| --- | --- |
| Windows | `liuguangtv-setup-VERSION-ARCH.exe`、`liuguangtv-portable-VERSION-windows-ARCH.zip` |
| macOS | `liuguangtv-VERSION-ARCH.dmg`、`liuguangtv-VERSION-darwin-ARCH.tar.gz` |
| Linux | `liuguangtv_VERSION_ARCH.deb`、`liuguangtv-VERSION-linux-ARCH.tar.gz` |

各平台还生成 `update-PLATFORM.json` 和可选的 `liuguangtv-FROM-to-VERSION-PLATFORM.delta`。原生 tar.gz 同时作为签名全量更新包，不再另外生成 macOS ZIP 或带 `app/` 的 Linux 便携包。不再按固定资产总数收集产物，避免遗漏可选增量包。

`build/package_release.py` 保留 MyGo 安装包及原生归档的原始字节，只额外生成 Windows 便携 ZIP：

- 根目录包含 `LiuguangTV.exe`、MyGo 的 `.mygo-files` 及其列出的安装文件、可选 `install.ps1` 和 `portable.flag`。
- ZIP 不含安装程序、更新归档、增量包、更新清单或用户数据；便携标记不会写入 MyGo 的安装包、全量或增量更新包。
- 用户数据位于 exe 同目录的 `data/`。旧 `app/` + 同级 `data/` + `liuguangtv-portable.mode` 布局继续由运行时识别，应用内更新后仍保持原数据路径，不会静默迁移。
- macOS/Linux 的新原生归档不附加便携标记，使用系统用户配置目录。手动从旧便携包改用新布局前应备份，并通过设置中的导入导出迁移。

所有产物都不附带 mpv，也没有播放器下载构建步骤。应用内“下载 mpv”只打开官方安装页面，用户自行安装并选择路径。

## 签名与增量更新

配置仓库 Secret `MYGO_UPDATER_PRIVATE_KEY`，与 `mygo.json` 的现有公钥匹配。不要打印、提交私钥或替换公钥，否则已安装版本无法验证后续更新。更新包签名不等于系统代码签名：Windows 安装程序没有发布者签名，macOS 使用临时签名且未经公证。

移除了 `updates.deltas: 0`，采用 MyGo 默认的增量更新策略。MyGo 根据已有发布版本生成增量包；首次发布、没有可用基线或增量不比全量包小时，可能不生成增量包，全量签名更新仍可使用。CI 向 MyGo 提供 GitHub token 以读取已发布的更新基线。

收集脚本根据每份清单的 `deltas` 收集引用的文件，缺失文件会失败；发布前逐个检查全量及增量包的版本、来源 URL、大小和 Ed25519 签名，并比较 Windows ZIP 与全量更新归档中的程序文件。测试使用临时生成的测试密钥，不接触生产签名私钥。

## 发布流程

1. 打标签前在 `CHANGELOG.md` 中新增唯一、非空的 `## MAJOR.MINOR.PATCH` 章节。MyGo 将章节内容写入更新清单；发布前检查六个平台的说明非空且一致。
2. 推送标签或手动选择兼容新打包布局的标签，CI 提前检查签名密钥与更新日志，然后创建草稿；已有同标签 Release 则复用。
3. 各平台完成构建、收集产物并生成各自的 `SHA256SUMS.txt`，使用 `gh release upload --clobber` 上传安装包、归档、清单、Windows ZIP 和可选增量包。
4. 所有平台及 Windows 原生测试成功后，下载 artifacts、校验哈希、全量与增量签名及便携内容，生成并上传一份汇总 `SHA256SUMS.txt`，更新 Release 说明并发布为 Latest。各任务自己的同名校验值文件只在 CI artifacts 中保留，避免互相覆盖。

同标签重跑会直接覆盖同名资产，包括已经发布的版本，不要求旧资产哈希或说明一致，也不自动删除无同名替代的额外资产。已发布版本重跑期间仍可见，资产可能暂时来自不同构建；正常更新仍建议发布新标签。旧布局标签不应由新工作流重新打包，新工作流会明确拒绝；需要重跑旧版时使用该版本原有工作流。

## 验证范围

本地验证包括 Go 测试、vet、编译、Python 打包测试和 Actionlint；测试覆盖双架构 Windows ZIP、原生归档不重打包、可选增量收集与验签、篡改和缺失产物、旧便携数据路径保持。

CI 和单元测试不能替代六个平台的真实安装、从历史版本应用增量升级、重启与数据保留验证。缺少生产密钥和目标系统桌面时，不应声称这些端到端流程已验证。旧版没有更新器时，仍需手动安装首个支持更新的版本。
