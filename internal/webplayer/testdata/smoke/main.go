package main

import (
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/webplayer"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run ./internal/webplayer/testdata/smoke <synthetic-media-directory>")
		os.Exit(2)
	}
	runtime.LockOSThread()
	server := httptest.NewServer(http.FileServer(http.Dir(os.Args[1])))
	defer server.Close()
	var window *mygo.Window
	var view *mygo.WebView
	var engine *webplayer.Player
	failed := false
	mygo.App.WhenReady(func() {
		window = mygo.NewWindow(mygo.WindowOptions{Title: "WebView playback smoke", Width: 700, Height: 450, Content: ui.View(func(c *ui.Context) {
			if view != nil {
				ui.WebView(c, view).Fill()
			}
		})})
		var err error
		view, err = window.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{PreloadScript: `document.addEventListener('DOMContentLoaded',()=>{const orig=mpegts.createPlayer;mpegts.createPlayer=(...args)=>{const p=orig(...args);p.on(mpegts.Events.ERROR,(...e)=>window.smokeErrors.push(JSON.stringify(e)));return p}});window.smokeErrors=[];const origError=console.error;console.error=(...args)=>{window.smokeErrors.push(args.join(' '));origError(...args)};const origFetch=window.fetch;window.fetch=(url,opt)=>{if(opt?.body)window.smokeLastReport=JSON.parse(opt.body);return origFetch(url,opt)}`}})
		if err != nil {
			panic(err)
		}
		engine, err = webplayer.New(view.Page().LoadURL)
		if err != nil {
			panic(err)
		}
		window.Invalidate()
		go func() {
			defer func() { engine.Close(); mygo.App.Quit() }()
			for _, format := range []string{"mp4", "m3u8", "flv", "ts"} {
				ready := make(chan struct{})
				progress := make(chan player.PlaybackStatus, 128)
				var once sync.Once
				done, err := engine.Play("", playlist.Channel{URL: server.URL + "/demo." + format}, player.Options{Controls: player.Controls{Volume: 70}, OnReady: func() { once.Do(func() { close(ready) }) }, OnPlayback: func(v player.PlaybackStatus) {
					select {
					case progress <- v:
					default:
					}
				}})
				if err != nil {
					fmt.Println("FAIL", format, err)
					failed = true
					return
				}
				select {
				case <-ready:
				case err := <-done:
					snapshot, _ := mygo.EvalAs[map[string]any](view.Page(), `({report:window.smokeLastReport,errors:window.smokeErrors})`)
					fmt.Println("FAIL", format, "ended", err, snapshot)
					failed = true
					return
				case <-time.After(20 * time.Second):
					snapshot, _ := mygo.EvalAs[map[string]any](view.Page(), `({failure:document.body.innerText,video:document.querySelector('video')?.error?.code,ready:document.querySelector('video')?.readyState,src:document.querySelector('video')?.currentSrc})`)
					fmt.Println("FAIL", format, "ready timeout", snapshot)
					failed = true
					return
				}
			drain:
				for {
					select {
					case <-progress:
					default:
						break drain
					}
				}
				engine.SetControls(player.Controls{Volume: 37, Muted: true, Paused: true})
				deadline := time.After(5 * time.Second)
				paused := false
				for !paused {
					select {
					case v := <-progress:
						paused = v.Paused != nil && *v.Paused
					case <-deadline:
						fmt.Println("FAIL", format, "pause")
						failed = true
						return
					}
				}
				actual, err := mygo.EvalAs[map[string]any](view.Page(), `({volume:document.querySelector('video').volume,muted:document.querySelector('video').muted,paused:document.querySelector('video').paused,width:document.querySelector('video').videoWidth})`)
				if err != nil || actual["volume"] != float64(.37) || actual["muted"] != true || actual["paused"] != true || actual["width"] != float64(320) {
					fmt.Println("FAIL", format, "controls", actual, err)
					failed = true
					return
				}
				engine.SetControls(player.Controls{Volume: 55})
				resumed := false
				deadline = time.After(5 * time.Second)
				for !resumed {
					select {
					case v := <-progress:
						resumed = v.Paused != nil && !*v.Paused && v.Position != nil && *v.Position > 0.3
					case <-deadline:
						fmt.Println("FAIL", format, "resume")
						failed = true
						return
					}
				}
				window.SetSize(850, 520)
				fmt.Println("PASS", format, "decode/pause/resume/volume/mute/resize", actual)
				engine.Stop()
				time.Sleep(300 * time.Millisecond)
				fmt.Println("STOP", format, view.Page().URL())
			}
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Println(err)
		failed = true
	}
	if failed {
		os.Exit(1)
	}
}
