//go:build windows

// Package embedhost owns a Windows child HWND reserved for mpv video output.
// All Host methods must be called on the MyGo UI/OS thread.
package embedhost

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	createWindow       = user32.NewProc("CreateWindowExW")
	destroyWindow      = user32.NewProc("DestroyWindow")
	setWindowPos       = user32.NewProc("SetWindowPos")
	showWindow         = user32.NewProc("ShowWindow")
	getDPI             = user32.NewProc("GetDpiForWindow")
	getWindow          = user32.NewProc("GetWindow")
	setWindowRgn       = user32.NewProc("SetWindowRgn")
	gdi32              = syscall.NewLazyDLL("gdi32.dll")
	createRectRgn      = gdi32.NewProc("CreateRectRgn")
	createRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	combineRgn         = gdi32.NewProc("CombineRgn")
	deleteObject       = gdi32.NewProc("DeleteObject")
)

const Supported = true

type Host struct {
	hwnd, parent uintptr
	last         [4]int32
	positioned   bool
	visible      bool
	region       [4]int32
	regionSet    bool
	overlay      [7]int32
	overlaySet   bool
}

func New(parent uintptr) (*Host, error) {
	if parent == 0 {
		return nil, fmt.Errorf("无法嵌入视频：主窗口句柄为空")
	}
	class, _ := syscall.UTF16PtrFromString("STATIC")
	title, _ := syscall.UTF16PtrFromString("")
	// Hidden until a stream starts; SS_BLACKRECT paints a black background.
	const style = 0x40000000 | 0x04000000 | 0x02000000 | 0x00000004 // CHILD | CLIPSIBLINGS | CLIPCHILDREN | SS_BLACKRECT
	hwnd, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), style, 0, 0, 1, 1, parent, 0, 0, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("创建视频子窗口失败：%w", err)
	}
	return &Host{hwnd: hwnd, parent: parent}, nil
}
func (h *Host) Handle() uintptr {
	if h == nil {
		return 0
	}
	return h.hwnd
}

// Place converts MyGo window-client DIPs to physical client pixels.
func (h *Host) Place(x, y, w, height float32) error {
	if h == nil || h.hwnd == 0 {
		return nil
	}
	scale := 1.0
	if getDPI.Find() == nil {
		dpi, _, _ := getDPI.Call(h.parent)
		if dpi > 0 {
			scale = float64(dpi) / 96
		}
	}
	left := int32(math.Round(float64(x) * scale))
	top := int32(math.Round(float64(y) * scale))
	right := int32(math.Round(float64(x+w) * scale))
	bottom := int32(math.Round(float64(y+height) * scale))
	rect := [4]int32{left, top, max(1, right-left), max(1, bottom-top)}
	if h.positioned && h.last == rect {
		return h.raise()
	}
	// HWND_TOP (0) places the host above MyGoSurface, its full-client GPU
	// sibling. SWP_NOZORDER here would leave decoded video hidden underneath.
	// SWP_NOACTIVATE still prevents stealing keyboard focus.
	ok, _, err := setWindowPos.Call(h.hwnd, 0, uintptr(rect[0]), uintptr(rect[1]), uintptr(rect[2]), uintptr(rect[3]), 0x0010)
	if ok == 0 {
		return fmt.Errorf("调整视频区域失败：%w", err)
	}
	h.last = rect
	h.positioned = true
	return nil
}

// raise changes only sibling z-order, never top-level always-on-top state.
func (h *Host) raise() error {
	above, _, _ := getWindow.Call(h.hwnd, 3) // GW_HWNDPREV
	if above == 0 {
		return nil
	}
	ok, _, err := setWindowPos.Call(h.hwnd, 0, 0, 0, 0, 0, 0x0010|0x0001|0x0002)
	if ok == 0 {
		return fmt.Errorf("调整视频窗口层级失败：%w", err)
	}
	return nil
}

// SetInsets clips the native child without resizing the video, exposing UI bars
// beneath it. Values are DIPs relative to the placed viewport.
func (h *Host) SetInsets(top, bottom float32) error {
	if h == nil || h.hwnd == 0 || !h.positioned {
		return nil
	}
	scale := 1.0
	if getDPI.Find() == nil {
		dpi, _, _ := getDPI.Call(h.parent)
		if dpi > 0 {
			scale = float64(dpi) / 96
		}
	}
	upper := max(0, int32(math.Round(float64(top)*scale)))
	lower := max(0, int32(math.Round(float64(bottom)*scale)))
	bounds := [4]int32{h.last[2], h.last[3], upper, lower}
	if h.regionSet && h.region == bounds {
		return nil
	}
	var region uintptr
	if upper > 0 || lower > 0 {
		region, _, _ = createRectRgn.Call(0, uintptr(min(upper, h.last[3])), uintptr(h.last[2]), uintptr(max(min(upper, h.last[3]), h.last[3]-lower)))
		if region == 0 {
			return fmt.Errorf("创建视频裁剪区域失败")
		}
	}
	ok, _, err := setWindowRgn.Call(h.hwnd, region, 1)
	if ok == 0 {
		if region != 0 {
			deleteObject.Call(region)
		}
		return fmt.Errorf("设置视频裁剪区域失败：%w", err)
	}
	h.region, h.regionSet = bounds, true
	h.overlaySet = false
	return nil
}

// SetOverlay exposes only a rounded UI control island. Coordinates are DIPs
// relative to the viewport; the video keeps its full size and aspect ratio.
// A zero-sized overlay restores the complete video region.
func (h *Host) SetOverlay(x, y, width, height, radius float32) error {
	if h == nil || h.hwnd == 0 || !h.positioned {
		return nil
	}
	if width <= 0 || height <= 0 {
		return h.SetInsets(0, 0)
	}
	scale := 1.0
	if getDPI.Find() == nil {
		if dpi, _, _ := getDPI.Call(h.parent); dpi > 0 {
			scale = float64(dpi) / 96
		}
	}
	px := func(v float32) int32 { return int32(math.Round(float64(v) * scale)) }
	bounds := [7]int32{h.last[2], h.last[3], px(x), px(y), px(x + width), px(y + height), max(0, px(radius*2))}
	if h.overlaySet && h.overlay == bounds {
		return nil
	}
	region, _, _ := createRectRgn.Call(0, 0, uintptr(bounds[0]), uintptr(bounds[1]))
	if region == 0 {
		return fmt.Errorf("创建视频裁剪区域失败")
	}
	// GDI's right/bottom coordinates are exclusive, as are the UI bounds.
	hole, _, _ := createRoundRectRgn.Call(uintptr(bounds[2]), uintptr(bounds[3]), uintptr(bounds[4]), uintptr(bounds[5]), uintptr(bounds[6]), uintptr(bounds[6]))
	if hole == 0 {
		deleteObject.Call(region)
		return fmt.Errorf("创建控制栏避让区域失败")
	}
	kind, _, _ := combineRgn.Call(region, region, hole, 4) // RGN_DIFF
	deleteObject.Call(hole)
	if kind == 0 {
		deleteObject.Call(region)
		return fmt.Errorf("合并视频裁剪区域失败")
	}
	ok, _, err := setWindowRgn.Call(h.hwnd, region, 1)
	if ok == 0 {
		deleteObject.Call(region)
		return fmt.Errorf("设置视频裁剪区域失败：%w", err)
	}
	// The system owns region after SetWindowRgn succeeds.
	h.overlay, h.overlaySet = bounds, true
	h.regionSet = false
	return nil
}

func (h *Host) Show(visible bool) {
	if h == nil || h.hwnd == 0 || h.visible == visible {
		return
	}
	cmd := uintptr(0)
	if visible {
		cmd = 8 // SW_SHOWNA: never activate or steal keyboard focus.
	}
	showWindow.Call(h.hwnd, cmd)
	if visible {
		_ = h.raise()
	}
	h.visible = visible
}
func (h *Host) Close() {
	if h == nil || h.hwnd == 0 {
		return
	}
	destroyWindow.Call(h.hwnd)
	h.hwnd = 0
}
