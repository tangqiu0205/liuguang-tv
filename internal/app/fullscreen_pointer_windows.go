//go:build windows

package app

import (
	"fmt"
	"syscall"
	"unsafe"
)

const fullscreenPointerSupported = true

var (
	pointerUser32         = syscall.NewLazyDLL("user32.dll")
	pointerForeground     = pointerUser32.NewProc("GetForegroundWindow")
	pointerGetCursor      = pointerUser32.NewProc("GetCursorPos")
	pointerScreenToClient = pointerUser32.NewProc("ScreenToClient")
	pointerGetClientRect  = pointerUser32.NewProc("GetClientRect")
)

type fullscreenPoint struct{ x, y int32 }
type fullscreenRect struct{ left, top, right, bottom int32 }

func fullscreenPointerPosition(window uintptr) (int32, int32, bool, error) {
	foreground, _, _ := pointerForeground.Call()
	if window == 0 || foreground != window {
		return 0, 0, false, nil
	}
	var screen fullscreenPoint
	if ok, _, err := pointerGetCursor.Call(uintptr(unsafe.Pointer(&screen))); ok == 0 {
		return 0, 0, false, fmt.Errorf(tr("读取鼠标位置失败：%w"), err)
	}
	client := screen
	if ok, _, err := pointerScreenToClient.Call(window, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return 0, 0, false, fmt.Errorf(tr("转换鼠标位置失败：%w"), err)
	}
	var rect fullscreenRect
	if ok, _, err := pointerGetClientRect.Call(window, uintptr(unsafe.Pointer(&rect))); ok == 0 {
		return 0, 0, false, fmt.Errorf(tr("读取全屏区域失败：%w"), err)
	}
	inside := client.x >= rect.left && client.x < rect.right && client.y >= rect.top && client.y < rect.bottom
	return screen.x, screen.y, inside, nil
}
