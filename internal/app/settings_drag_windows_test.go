//go:build windows

package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func init() {
	if os.Getenv("LIUGUANG_TV_SETTINGS_DRAG_CHILD") == "1" {
		if err := nativeSettingsDragChild(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}

// An isolated native window verifies that blank header/branding presses reach
// Windows StartDrag. Intercept HTCAPTION before the OS moves the window, so
// this test never moves the user's pointer or starts a desktop drag loop.
func TestNativeSettingsHeaderDrag(t *testing.T) {
	if os.Getenv("LIUGUANG_TV_TEST_SETTINGS_DRAG") != "1" {
		t.Skip("set LIUGUANG_TV_TEST_SETTINGS_DRAG=1 to test native drag regions")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeSettingsHeaderDrag$")
	cmd.Env = append(os.Environ(), "LIUGUANG_TV_SETTINGS_DRAG_CHILD=1", "LIUGUANG_TV_SETTINGS_DRAG_DATA="+t.TempDir(), "MYGO_GPU=0", "MYGO_LOCALE=zh-Hans")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native settings drag failed: %v\n%s", err, output)
	}
}

func nativeSettingsDragChild() error {
	user32 := syscall.NewLazyDLL("user32.dll")
	setProc := user32.NewProc("SetWindowLongPtrW")
	callProc := user32.NewProc("CallWindowProcW")
	send := user32.NewProc("SendMessageW")
	find := user32.NewProc("FindWindowExW")
	getDPI := user32.NewProc("GetDpiForWindow")
	mygo.App.SetPath(mygo.PathUserData, os.Getenv("LIUGUANG_TV_SETTINGS_DRAG_DATA"))
	mygo.App.SetLanguages("zh-Hans", "en")
	a := &application{showSettings: true, state: store.State{ThemeMode: "light"}}
	var issue error
	finished := false
	mygo.App.WhenReady(func() {
		a.window = mygo.NewWindow(a.mainWindowOptions())
		hwnd := a.window.NativeHandle()
		var original uintptr
		drags := 0
		callback := syscall.NewCallback(func(h, message, wp, lp uintptr) uintptr {
			if message == 0x00A1 && wp == 2 { // WM_NCLBUTTONDOWN, HTCAPTION
				drags++
				return 0
			}
			result, _, _ := callProc.Call(original, h, message, wp, lp)
			return result
		})
		original, _, issue = setProc.Call(hwnd, ^uintptr(3), callback) // GWLP_WNDPROC = -4
		if original == 0 {
			mygo.App.Quit()
			return
		}
		issue = nil
		a.window.OnReadyToShow(func() {
			time.AfterFunc(50*time.Millisecond, func() {
				a.window.Update(func() {
					defer mygo.App.Quit()
					defer setProc.Call(hwnd, ^uintptr(3), original)
					class, _ := syscall.UTF16PtrFromString("MyGoSurface")
					surface, _, _ := find.Call(hwnd, 0, uintptr(unsafe.Pointer(class)), 0)
					if surface == 0 {
						issue = fmt.Errorf("native UI surface not found")
						return
					}
					dpi, _, _ := getDPI.Call(hwnd)
					for _, point := range [][2]uintptr{{400, 12}, {50, 50}} {
						x, y := point[0]*dpi/96, point[1]*dpi/96
						before := drags
						send.Call(surface, 0x0201, 1, x|(y<<16)) // WM_LBUTTONDOWN
						send.Call(surface, 0x0202, 0, x|(y<<16)) // WM_LBUTTONUP
						if drags != before+1 {
							issue = fmt.Errorf("point %v did not initiate a window drag", point)
							return
						}
					}
					finished = true
				})
			})
		})
		a.window.Show()
	})
	timeout := time.AfterFunc(8*time.Second, func() { mygo.App.Quit() })
	defer timeout.Stop()
	if err := mygo.App.Run(); err != nil {
		return err
	}
	if issue != nil {
		return issue
	}
	if !finished {
		return fmt.Errorf("native drag check did not complete")
	}
	return nil
}
