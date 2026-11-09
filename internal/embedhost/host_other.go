//go:build !windows

package embedhost

const Supported = false

type Host struct{}

func New(parent uintptr) (*Host, error)                              { return &Host{}, nil }
func (h *Host) Handle() uintptr                                      { return 0 }
func (h *Host) Place(x, y, w, height float32) error                  { return nil }
func (h *Host) SetInsets(top, bottom float32) error                  { return nil }
func (h *Host) SetOverlay(x, y, width, height, radius float32) error { return nil }
func (h *Host) Show(visible bool)                                    {}
func (h *Host) Close()                                               {}
