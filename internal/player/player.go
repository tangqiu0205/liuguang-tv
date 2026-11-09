// Package player controls one mpv process without invoking a shell.
package player

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

type Controls struct {
	Volume        float64
	Muted, Paused bool
}
type Options struct {
	Window         uintptr
	Placement      *WindowPlacement // Standalone window geometry, applied only at process startup.
	Controls       Controls
	OnControlError func(error)          // Called on the IPC worker, never on the UI thread.
	OnReady        func()               // Playback timeline confirmed over IPC; called off the UI thread.
	OnPlayback     func(PlaybackStatus) // Continuous progress/pause/buffering observations.
}
type session struct {
	cmd         *exec.Cmd
	pipe        string
	updates     chan Controls
	stopped     chan struct{}
	intentional atomic.Bool
}
type Player struct {
	Logger  *diagnostic.Logger
	mu      sync.Mutex
	current *session
}

var serial atomic.Uint64

func Resolve(configured string) (string, error) {
	if configured != "" {
		p, err := exec.LookPath(configured)
		if err != nil {
			return "", fmt.Errorf("找不到指定的 mpv 程序")
		}
		return p, nil
	}
	name := "mpv"
	if runtime.GOOS == "windows" {
		name = "mpv.exe"
	}
	if exe, err := os.Executable(); err == nil {
		path := filepath.Join(filepath.Dir(exe), name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		root := os.Getenv("SCOOP")
		if root == "" {
			if home, e := os.UserHomeDir(); e == nil {
				root = filepath.Join(home, "scoop")
			}
		}
		if root != "" {
			candidate := filepath.Join(root, "apps", "mpv", "current", "mpv.exe")
			if info, e := os.Stat(candidate); e == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("未找到 mpv：请安装 mpv，或填写 mpv.exe 的完整路径")
}

func arguments(ch playlist.Channel, opt Options, pipe string) []string {
	args := []string{"--no-config", "--load-scripts=no", "--ytdl=no", "--idle=no", "--force-window=yes", "--title=" + ch.Name,
		"--input-terminal=no", "--volume=" + fmt.Sprint(max(0, min(100, opt.Controls.Volume)))}
	if ch.Headers.UserAgent != "" {
		args = append(args, "--user-agent="+ch.Headers.UserAgent)
	}
	if ch.Headers.Referer != "" {
		args = append(args, "--referrer="+ch.Headers.Referer)
	}
	if opt.Controls.Muted {
		args = append(args, "--mute=yes")
	}
	if opt.Controls.Paused {
		args = append(args, "--pause=yes")
	}
	if opt.Window != 0 {
		// mpv's win32 --wid expects the HWND cast to uint32, not a signed number.
		args = append(args, fmt.Sprintf("--wid=%d", uint32(opt.Window)), "--osc=no", "--input-default-bindings=no", "--input-vo-keyboard=no")
	}
	if opt.Window == 0 && opt.Placement != nil {
		args = append(args, opt.Placement.arguments(runtime.GOOS)...)
	}
	if pipe != "" {
		args = append(args, "--input-ipc-server="+pipe)
	}
	return append(args, "--", ch.URL)
}

// Play starts a stream, stops the preceding process, and reports process exit.
func (p *Player) Play(binary string, ch playlist.Channel, options ...Options) (doneResult <-chan error, playErr error) {
	defer func() { p.Logger.Record(diagnostic.PlayerStart, ch.URL, playErr) }()
	if !playlist.ValidStream(ch.URL) {
		return nil, fmt.Errorf("不支持该播放地址")
	}
	if err := ch.Headers.Validate(); err != nil {
		return nil, err
	}
	opt := Options{Controls: Controls{Volume: 70}}
	if len(options) > 0 {
		opt = options[0]
	}
	path, err := Resolve(binary)
	if err != nil {
		return nil, err
	}
	pipe := ""
	if runtime.GOOS == "windows" {
		pipe = fmt.Sprintf(`\\.\pipe\liuguang-tv-%d-%d`, os.Getpid(), serial.Add(1))
	}
	s := &session{pipe: pipe, updates: make(chan Controls, 1), stopped: make(chan struct{})}
	s.cmd = exec.Command(path, arguments(ch, opt, pipe)...)
	configureProcess(s.cmd)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err = s.cmd.Start(); err != nil {
		return nil, fmt.Errorf("无法启动 mpv: %w", err)
	}
	if p.current != nil {
		p.current.intentional.Store(true)
		_ = p.current.cmd.Process.Kill()
	}
	p.current = s
	done := make(chan error, 1)
	go func() {
		err := s.cmd.Wait()
		if s.intentional.Load() {
			p.Logger.Record(diagnostic.PlayerStop, ch.URL, nil)
		} else {
			p.Logger.Record(diagnostic.PlayerExit, ch.URL, err)
		}
		close(s.stopped)
		p.mu.Lock()
		if p.current == s {
			p.current = nil
		}
		p.mu.Unlock()
		done <- err
		close(done)
	}()
	if pipe != "" {
		go s.controlLoop(opt.OnControlError)
		if opt.OnReady != nil || opt.OnPlayback != nil {
			go s.monitorPlayback(opt.OnReady, opt.OnPlayback)
		}
	}
	return done, nil
}

// SetControls coalesces rapid slider edits instead of spawning a goroutine per edit.
func (p *Player) SetControls(c Controls) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil || p.current.pipe == "" {
		return
	}
	select {
	case p.current.updates <- c:
	default:
		select {
		case <-p.current.updates:
		default:
		}
		p.current.updates <- c
	}
}
func (s *session) controlLoop(onError func(error)) {
	for {
		select {
		case <-s.stopped:
			return
		case c := <-s.updates:
			err := s.send(c)
			if err != nil && onError != nil {
				select {
				case <-s.stopped:
					return
				default:
					onError(err)
				}
			}
		}
	}
}
func (s *session) send(c Controls) error {
	// The pipe may not exist during mpv startup. Retry only for a bounded period.
	deadline := time.Now().Add(3 * time.Second)
	for {
		select {
		case <-s.stopped:
			return nil
		default:
		}
		f, err := openPipe(s.pipe)
		if err == nil {
			err = writeControls(f, c)
			_ = f.Close()
			if err == nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("mpv 控制连接失败，请检查播放器版本或重新播放")
		}
		select {
		case <-s.stopped:
			return nil
		case <-time.After(80 * time.Millisecond):
		}
	}
}
func writeControls(w io.Writer, c Controls) error {
	enc := json.NewEncoder(w)
	for _, command := range [][]any{
		{"set_property", "volume", max(0, min(100, c.Volume))},
		{"set_property", "mute", c.Muted},
		{"set_property", "pause", c.Paused},
	} {
		if err := enc.Encode(map[string]any{"command": command}); err != nil {
			return err
		}
	}
	return nil
}
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current != nil {
		p.current.intentional.Store(true)
		_ = p.current.cmd.Process.Kill()
		p.current = nil
	}
}
