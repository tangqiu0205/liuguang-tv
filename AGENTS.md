# 项目指南

## 项目结构

- 流光 TV 是使用 Go 和 MyGo 原生 UI 的 IPTV 客户端，支持 mpv 和内置 WebView 两种播放引擎。入口为 `cmd/liuguangtv`。
- `internal/app`：界面、播放协调、订阅、EPG、设置及更新入口。
- `internal/player`、`internal/embedhost`、`internal/webplayer`：播放器与平台集成。
- `internal/store`：配置及数据路径；`internal/playlist`、`internal/epg`：频道和节目单；`internal/l10n`：界面语言。
- `build/` 和 `.github/workflows/`：图标、打包、签名校验及发布。

## 开发与验证

- 只新增覆盖当前改动关键行为、边界或缺陷的必要测试；优先复用现有测试，避免重复测试和仅验证实现细节的测试。
- Go、MyGo 及 CLI 版本以 `go.mod` 为准；使用 `go tool mygo dev` 开发，原生窗口需要图形桌面环境。
- 常规验证：`go test ./...`、`go vet ./...`、`go build ./...`。
- 修改打包或工作流时运行 `python -m unittest discover -s build -p 'test_*.py' -v`，并在可用时用 actionlint 检查工作流。
- Windows 原生窗口测试为显式 opt-in，按测试文件中的 `LIUGUANG_TV_TEST_*` 环境变量运行。未执行的跨平台、图形交互及原生测试须明确说明；无头测试不能替代真实桌面验证。

## 项目约束

- 不预置节目源，不捆绑或自动下载安装 mpv；应用只提供官方安装页面，由用户自行安装。
- 界面支持跟随系统、简体中文和 English；修改界面文案时同步维护 `internal/l10n/en.json` 并检查中英文布局，不翻译用户频道、订阅或节目名称。详见 `docs/I18N.md`。
- Windows 便携版使用 `portable.flag` 和程序目录的 `data/`；保留旧版 `app/` + 同级 `data/` 布局兼容，不静默迁移、覆盖或删除用户数据。
- 检查 Windows、macOS 和 Linux 的平台实现及构建约束；调整第三方依赖或分发内容时，同步检查 `THIRD_PARTY_NOTICES.md` 和许可材料。
- 保留更新包签名验证和现有公钥，不打印、提交私钥或擅自更换公钥。普通开发构建不具备正式更新能力。
- `README.md` 面向最终用户；隐私、许可和更新日志分别维护在 `docs/PRIVACY.md`、`THIRD_PARTY_NOTICES.md` 和 `CHANGELOG.md`。

## 发布

- 以 `vMAJOR.MINOR.PATCH` 标签作为正式版本输入；发布前在 `CHANGELOG.md` 新增对应版本章节。CI 临时更新 `mygo.json` 版本并注入程序版本，不提交该临时变更。详见 `docs/RELEASING.md`。
- 推送版本标签会触发发布流程；同标签重跑允许覆盖已发布的同名资产。提交、推送、打标签、发布或修改远端历史前遵循用户授权，不主动改写已发布标签。
- Windows 安装包未使用发布者签名，macOS 包未经公证；更新包签名与系统代码签名是两回事。CI 不能替代真实安装及历史版本升级验证。
