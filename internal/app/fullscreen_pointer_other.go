//go:build !windows

package app

const fullscreenPointerSupported = false

func fullscreenPointerPosition(uintptr) (int32, int32, bool, error) {
	return 0, 0, false, nil
}
