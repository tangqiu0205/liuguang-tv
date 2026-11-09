# 第三方许可说明

本文件不为 LiuguangTV 自身指定许可证。项目代码尚未声明开源许可证，需由权利人另行确认。

## Go 依赖

直接依赖 [MyGo v0.8.3](https://github.com/egoist/mygo/tree/v0.8.3)，使用 MIT 许可证，Copyright (c) 2026 MyGo contributors。

完整直接和间接模块版本以 [go.mod](go.mod) 与 [go.sum](go.sum) 为准；上述说明不是全部依赖的许可证清单。再分发前应逐一核对其许可证和署名要求。

## 内嵌媒体脚本

| 组件 | 版本 | 许可证与上游 |
| --- | --- | --- |
| hls.js | 1.7.3 | Apache-2.0，[上游](https://github.com/video-dev/hls.js)、[许可与署名](internal/webplayer/assets/vendor/hls-LICENSE.txt) |
| mpegts.js | 1.8.2 | Apache-2.0，[上游](https://github.com/xqq/mpegts.js)、[完整许可](internal/webplayer/assets/vendor/mpegts-LICENSE.txt) |

脚本及许可证随 WebView 资源内嵌。固定下载来源与 SHA-256 见 [WebView 文档](docs/WEBVIEW.md)。不要将 vendor 文件当成无关构建产物删除。

## 外部播放器与系统组件

mpv 不随本项目分发，用户自行安装，其许可由对应发行包和构建方式决定。WebView2、WKWebView、WebKitGTK 由对应平台或用户提供，受各自条款约束。

发布工具依赖（如图标渲染工具）与应用运行依赖不同；其版本和安装方式见 `build/` 与 GitHub Actions 工作流。

## 再分发检查

公开源码不自动满足二进制分发义务。发布前应核对 Go 依赖、内嵌脚本、图标及产物中的许可文本和署名；不要以删除开发历史为由移除必要的上游版权信息。
