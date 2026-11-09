//go:build windows

package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

// Opt-in real mpv integration test. Uses only a locally generated GIF over HTTP;
// no live TV sources, Internet media, or persistent user configuration.
func TestWindowsEmbeddedMPV(t *testing.T) {
	binary := os.Getenv("LIUGUANG_TV_TEST_MPV")
	if binary == "" {
		t.Skip("set LIUGUANG_TV_TEST_MPV to an mpv.exe path for native integration")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user32 := syscall.NewLazyDLL("user32.dll")
	class, _ := syscall.UTF16PtrFromString("STATIC")
	parent, _, err := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0, 0x00CF0000, 0, 0, 800, 600, 0, 0, 0, 0)
	if parent == 0 {
		t.Fatal(err)
	}
	defer user32.NewProc("DestroyWindow").Call(parent)
	host, err := embedhost.New(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	if err := host.Place(20, 40, 640, 360); err != nil {
		t.Fatal(err)
	}
	host.Show(true)
	animation := &gif.GIF{LoopCount: 0}
	palette := color.Palette{color.Black, color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}}
	for i := 0; i < 60; i++ {
		frame := image.NewPaletted(image.Rect(0, 0, 160, 90), palette)
		for p := range frame.Pix {
			frame.Pix[p] = uint8(1 + i%2)
		}
		animation.Image = append(animation.Image, frame)
		animation.Delay = append(animation.Delay, 50)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		_ = gif.EncodeAll(w, animation)
	}))
	defer server.Close()
	var p Player
	ready := make(chan struct{}, 1)
	progress := make(chan PlaybackStatus, 128)
	done, err := p.Play(binary, playlist.Channel{Name: "Local embed integration", URL: server.URL + "/test.gif"}, Options{
		Window: host.Handle(), Controls: Controls{Volume: 65},
		OnReady: func() { ready <- struct{}{} },
		OnPlayback: func(status PlaybackStatus) {
			select {
			case progress <- status:
			default:
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()
	// Pump the owning UI thread: embedding can send synchronous Win32 messages.
	peek := user32.NewProc("PeekMessageW")
	translate := user32.NewProc("TranslateMessage")
	dispatch := user32.NewProc("DispatchMessageW")
	var msg struct {
		Hwnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		X, Y    int32
		Private uint32
	}
	pump := func() {
		for {
			ok, _, _ := peek.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
			if ok == 0 {
				break
			}
			translate.Call(uintptr(unsafe.Pointer(&msg)))
			dispatch.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
	deadline := time.Now().Add(8 * time.Second)
	var video uintptr
	for time.Now().Before(deadline) {
		pump()
		select {
		case err := <-done:
			t.Fatalf("mpv exited before embedding: %v", err)
		default:
		}
		video, _, _ = user32.NewProc("FindWindowExW").Call(host.Handle(), 0, 0, 0)
		if video != 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if video == 0 {
		t.Fatal("mpv did not create a child of the reserved video HWND")
	}
	// Query IPC while continuing to pump messages on the window-owning thread.
	type result struct {
		value any
		err   error
	}
	query := func(property string) any {
		t.Helper()
		p.mu.Lock()
		pipe := p.current.pipe
		p.mu.Unlock()
		resultCh := make(chan result, 1)
		go func() { value, err := queryProperty(pipe, property); resultCh <- result{value, err} }()
		limit := time.After(5 * time.Second)
		for {
			pump()
			select {
			case r := <-resultCh:
				if r.err != nil {
					t.Fatal(r.err)
				}
				return r.value
			case <-limit:
				t.Fatalf("IPC query %s timed out", property)
				return nil
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	// Wait until the synthetic video is decoded, not merely until the process starts.
	for time.Now().Before(deadline) {
		if params, ok := query("video-params").(map[string]any); ok && params["w"] == float64(160) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if params, ok := query("video-params").(map[string]any); !ok || params["w"] != float64(160) {
		t.Fatalf("no decoded test video: %+v", params)
	}
	readyDeadline := time.Now().Add(3 * time.Second)
	observed := false
	for !observed && time.Now().Before(readyDeadline) {
		pump()
		select {
		case <-ready:
			observed = true
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if !observed {
		t.Fatal("IPC readiness monitor did not confirm playing media")
	}
	p.SetControls(Controls{Volume: 35, Muted: true, Paused: true})
	for time.Now().Before(deadline) {
		if query("pause") == true && query("mute") == true && query("volume") == float64(35) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if query("pause") != true || query("mute") != true || query("volume") != float64(35) {
		t.Fatal("embedded playback controls were not applied")
	}
	waitStatus := func(match func(PlaybackStatus) bool) {
		t.Helper()
		limit := time.Now().Add(3 * time.Second)
		for time.Now().Before(limit) {
			pump()
			select {
			case status := <-progress:
				if match(status) {
					return
				}
			case <-done:
				t.Fatal("mpv exited during progress monitoring")
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
		t.Fatal("continuous playback observation timed out")
	}
	waitStatus(func(s PlaybackStatus) bool { return s.Paused != nil && *s.Paused })
	p.SetControls(Controls{Volume: 35, Muted: true, Paused: false})
	waitStatus(func(s PlaybackStatus) bool { return s.Paused != nil && !*s.Paused })
	firstPosition := -1.0
	waitStatus(func(s PlaybackStatus) bool {
		if s.Position == nil {
			return false
		}
		if firstPosition < 0 {
			firstPosition = *s.Position
			return false
		}
		return *s.Position > firstPosition
	})
	if err := host.Place(30, 50, 480, 270); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		pump()
		time.Sleep(10 * time.Millisecond)
	}
	var rect struct{ Left, Top, Right, Bottom int32 }
	ok, _, _ := user32.NewProc("GetClientRect").Call(video, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 || rect.Right <= 0 || rect.Bottom <= 0 {
		t.Fatal("invalid embedded video rectangle")
	}
	var hostRect struct{ Left, Top, Right, Bottom int32 }
	user32.NewProc("GetClientRect").Call(host.Handle(), uintptr(unsafe.Pointer(&hostRect)))
	if rect.Right != hostRect.Right || rect.Bottom != hostRect.Bottom {
		t.Fatalf("mpv child did not follow host resize: child=%+v host=%+v", rect, hostRect)
	}
	if err := host.SetInsets(40, 40); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		pump()
		time.Sleep(10 * time.Millisecond)
	}
	var clippedRect struct{ Left, Top, Right, Bottom int32 }
	user32.NewProc("GetClientRect").Call(video, uintptr(unsafe.Pointer(&clippedRect)))
	if clippedRect != rect {
		t.Fatalf("clipping controls resized video: before=%+v after=%+v", rect, clippedRect)
	}
	if err := host.SetInsets(0, 0); err != nil {
		t.Fatal(err)
	}
	t.Logf("decoded 160x90 video inside HWND %x; pause/resume, continuous timeline, mute, volume, resize and clipping verified", host.Handle())
	p.Stop()
	for i := 0; i < 150; i++ {
		pump()
		select {
		case <-done:
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatal("mpv did not stop")
}

func queryProperty(pipe, property string) (any, error) {
	name, err := syscall.UTF16PtrFromString(pipe)
	if err != nil {
		return nil, err
	}
	var h syscall.Handle
	deadline := time.Now().Add(2 * time.Second)
	for {
		h, err = syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, 0x00100000|0x00010000, 0)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(20 * time.Millisecond)
	}
	f := os.NewFile(uintptr(h), pipe)
	defer f.Close()
	if err := json.NewEncoder(f).Encode(map[string]any{"command": []any{"get_property", property}, "request_id": 1}); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bufio.NewReader(f))
	for {
		var reply struct {
			RequestID int    `json:"request_id"`
			Data      any    `json:"data"`
			Error     string `json:"error"`
		}
		if err := decoder.Decode(&reply); err != nil {
			return nil, err
		}
		if reply.RequestID == 1 {
			if reply.Error != "success" && reply.Error != "property unavailable" {
				return nil, fmt.Errorf("IPC: %s", reply.Error)
			}
			return reply.Data, nil
		}
	}
}
