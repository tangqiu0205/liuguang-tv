//go:build windows

package embedhost

import (
	"math"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestHostAboveUISurface(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := syscall.UTF16PtrFromString("STATIC")
	makeWindow := func(parent uintptr, style uintptr) uintptr {
		t.Helper()
		hwnd, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(class)), 0, style, 0, 0, 640, 480, parent, 0, 0, 0)
		if hwnd == 0 {
			t.Fatal(err)
		}
		return hwnd
	}
	parent := makeWindow(0, 0x00CF0000)
	defer destroyWindow.Call(parent)
	// Like MyGoSurface, a visible GPU child covers the whole client area.
	surface := makeWindow(parent, 0x40000000|0x10000000|0x04000000)
	h, err := New(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := h.Place(20, 40, 400, 240); err != nil {
		t.Fatal(err)
	}
	h.Show(true)
	previous := user32.NewProc("GetWindow")
	assertTop := func() {
		t.Helper()
		above, _, _ := previous.Call(h.Handle(), 3) // GW_HWNDPREV: sibling above the video host.
		if above != 0 {
			t.Fatalf("video host is obscured by sibling HWND %x (UI surface %x)", above, surface)
		}
	}
	assertTop()
	// Recreate the problem without moving/resizing the viewport: the rendering
	// surface is brought above it, then Place must restore the host's z-order.
	setWindowPos.Call(surface, 0, 0, 0, 0, 0, 0x0010|0x0001|0x0002)
	if err := h.Place(20, 40, 400, 240); err != nil {
		t.Fatal(err)
	}
	assertTop()
	h.Show(false)
	h.Show(true)
	assertTop()
	if err := h.Place(30, 50, 480, 270); err != nil {
		t.Fatal(err)
	}
	assertTop()
	if err := h.SetInsets(30, 40); err != nil {
		t.Fatal(err)
	}
	region, _, _ := createRectRgn.Call(0, 0, 0, 0)
	if region == 0 {
		t.Fatal("cannot create test region")
	}
	defer deleteObject.Call(region)
	getRegion := user32.NewProc("GetWindowRgn")
	getBox := gdi32.NewProc("GetRgnBox")
	if kind, _, _ := getRegion.Call(h.Handle(), region); kind == 0 {
		t.Fatal("window region missing")
	}
	var rect struct{ left, top, right, bottom int32 }
	if kind, _, _ := getBox.Call(region, uintptr(unsafe.Pointer(&rect))); kind == 0 {
		t.Fatal("cannot read region")
	}
	scale := 1.0
	if getDPI.Find() == nil {
		if dpi, _, _ := getDPI.Call(parent); dpi > 0 {
			scale = float64(dpi) / 96
		}
	}
	if rect.left != 0 || rect.right != h.last[2] || rect.top != int32(math.Round(30*scale)) || rect.bottom != h.last[3]-int32(math.Round(40*scale)) {
		t.Fatalf("unexpected video clip: %+v, host %+v", rect, h.last)
	}
	// A floating control island must not clip a full-width band: preserve
	// video beside it, below it, and in its rounded corners.
	for i := 0; i < 2; i++ { // Exercise both installation and cached frames.
		if err := h.SetOverlay(100, 180, 280, 56, 16); err != nil {
			t.Fatal(err)
		}
		if kind, _, _ := getRegion.Call(h.Handle(), region); kind == 0 {
			t.Fatal("floating control region missing")
		}
		pointInRegion := gdi32.NewProc("PtInRegion")
		for _, point := range []struct {
			x, y    float64
			visible bool
		}{
			{240, 208, false}, // Control center is exposed to the UI.
			{20, 208, true},   // Video to the left.
			{450, 208, true},  // Video to the right.
			{240, 250, true},  // Video below the bar.
			{101, 181, true},  // Video outside the rounded corner.
			{240, 100, true},  // Video above the bar.
		} {
			inside, _, _ := pointInRegion.Call(region, uintptr(int32(math.Round(point.x*scale))), uintptr(int32(math.Round(point.y*scale))))
			if (inside != 0) != point.visible {
				t.Fatalf("unexpected video visibility at (%v, %v): %v", point.x, point.y, inside != 0)
			}
		}
	}
	if err := h.SetOverlay(0, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if kind, _, _ := getRegion.Call(h.Handle(), region); kind != 0 {
		t.Fatal("floating control clip not removed")
	}
	if err := h.SetInsets(0, 0); err != nil {
		t.Fatal(err)
	}
	if kind, _, _ := getRegion.Call(h.Handle(), region); kind != 0 {
		t.Fatal("video clip not removed")
	}
}
