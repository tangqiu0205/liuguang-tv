# WebView 播放

## 结构与边界

- 默认 **mpv 播放器**，可在设置 → 播放器切换到 **内置 WebView**。不称“外部播放器”，因为 Windows 的 mpv 虽是独立进程，画面仍嵌入应用；也不支持任意播放器的命令行协议。
- 原生窗口通过 MyGo `Window.NewWebView` / `ui.WebView` 放置视频页面，频道、设置、控制仍是 Native UI。
- `internal/webplayer` 内嵌网页资源，按播放会话提供受限的本机 HTTP 转发；页面每 250ms 上报时间轴/暂停/缓冲/失败并获取控制值，不依赖第三方网页或公开 Go 绑定。
- 主清单、子清单、密钥、初始化片段、媒体分片及字节范围都通过同源转发；请求头从当前线路继承，不转发浏览器 Cookie、上游 Set-Cookie 或任意请求头。私网 IPTV 地址允许使用；只能访问当前用户所选线路及其清单引用的 HTTP(S) 资源，不提供开放代理。
- 精确校验 Host 和 Origin，随机页面/会话令牌、CSP、禁止页面导航；停止后撤销会话和媒体连接并导航空白页，不能仅隐藏 WebView 留下声音。
- 切回 mpv（包括导入 mpv 配置）时，关闭本机服务、销毁 WebView 并清空实例引用；再次选择 WebView 时不立即创建，直到播放频道才重新初始化。普通停止/切台仍复用 WebView。系统浏览器的共享进程与缓存不保证立即全部释放。
- 空闲时可调整静音/音量，只更新控制值，不提前创建引擎；下次播放继承这些控制值。首次选择、手动选线及自动切线共用地址兼容检查，不兼容协议计入已跳过集合并尝试下一条；不会因此提前终止兼容线路的尝试，也不会绕过已启用源范围。真正的引擎启动失败仍终止，不反复扫描线路。
- HLS 单份清单上限 4 MiB，引用映射上限 8192，容量回收只删除超过 10 分钟未使用的非根引用；支持长期滚动清单，不按累计分片数限制观看时长。超限明确失败，不跳过分片。
- HTML video 承担最终解码，JS 库主要做协议/封装适配，并非 FFmpeg 或通用软解码器。WebView 不支持的协议或编解码应主动选择 mpv，不自动换引擎。HLS 变量替换、Content Steering 不支持，遇到时显式失败。

## 随程序分发的依赖

媒体脚本来自各自上游，许可证文本内嵌在 `internal/webplayer/assets/vendor/`。完整路径及许可索引见 [第三方许可说明](../THIRD_PARTY_NOTICES.md)。

| 文件 | 固定版本 | 来源 | SHA-256 |
| --- | --- | --- | --- |
| `hls.min.js` | 1.7.3 | https://cdn.jsdelivr.net/npm/hls.js@1.7.3/dist/hls.min.js | `a12e7ee1cd64a69dcdb314157e45dafcba705bfb0b1440b7935cb265d374423e` |
| `mpegts.js` | 1.8.2 | https://cdn.jsdelivr.net/npm/mpegts.js@1.8.2/dist/mpegts.js | `bda31748736a69cb610c2edf4623e633f1f4f47b5bda83668c8d287e51b0c3a8` |

- hls.js：https://github.com/video-dev/hls.js ，Apache-2.0。
- mpegts.js：https://github.com/xqq/mpegts.js ，Apache-2.0。
- 使用源包随附许可证，Apache 完整文本亦随 mpegts.js 的 `mpegts-LICENSE.txt` 分发。

更新依赖时重新下载固定版本、核对许可证和哈希，然后执行自动化及真实 WebView 冒烟；运行时不访问 CDN。

## 测试覆盖与复测

自动化覆盖引擎设置保存/备份、默认 mpv、路径显隐及保留、切换停止旧会话、共享启动/断流检测、令牌失效、Host/Origin、非法协议、清单相对地址/重定向/密钥/Map 重写、请求头/Range、连接取消、映射回收和资源离线内嵌。

生命周期回归覆盖手动选择 mpv、导入默认/显式 mpv 配置及退出时关闭监听服务、结束会话、清空引用，以及普通停止/仍选 WebView 时保留复用。Windows 可选原生测试检查真实 WebView 销毁、旧服务不可复用、再次播放路径创建新实例；需在目标环境显式执行，不据此声称浏览器共享进程内存立即归还。

```powershell
$env:LIUGUANG_TV_TEST_WEBVIEW_LIFECYCLE = '1'
go test ./internal/app -run '^TestNativeWebViewReleaseAndRecreate$' -count=1 -v
Remove-Item Env:LIUGUANG_TV_TEST_WEBVIEW_LIFECYCLE
```

Windows WebView2 冒烟工具使用本地合成 H.264/AAC 视频，检查 MP4、HLS、HTTP-FLV、HTTP-TS 解码，以及实际 `HTMLVideoElement` 的音量、静音、暂停/恢复、视频尺寸和停止导航。FLV/TS worker 必须接收绝对媒体 URL，此约束有回归测试。

安装可信的 ffmpeg 后，在项目根目录使用 Git Bash 生成不含真实频道的测试素材：

```bash
mkdir -p dist/webview-smoke
ffmpeg -y -f lavfi -i testsrc2=size=320x180:rate=25 -f lavfi -i sine=frequency=440:sample_rate=44100 -t 24 -c:v libx264 -preset ultrafast -g 50 -pix_fmt yuv420p -c:a aac -movflags +faststart dist/webview-smoke/demo.mp4
ffmpeg -y -i dist/webview-smoke/demo.mp4 -c copy -hls_time 2 -hls_list_size 0 dist/webview-smoke/demo.m3u8
ffmpeg -y -i dist/webview-smoke/demo.mp4 -c copy -f flv dist/webview-smoke/demo.flv
ffmpeg -y -i dist/webview-smoke/demo.mp4 -c copy -f mpegts dist/webview-smoke/demo.ts
go run ./internal/webplayer/testdata/smoke dist/webview-smoke
```

此命令会弹出真实原生/WebView 窗口，失败返回非零退出码；不会读取用户配置或访问真实电视源。它不属于普通 `go test ./...`，需要图形桌面和系统 WebView。

尚未验证：真实源成功率、H.265/AC-3 等编解码组合、长时播放、真实断网/休眠、多屏跨 DPI、macOS/Linux；不要将本机合成视频短时通过等同于上述场景通过。
