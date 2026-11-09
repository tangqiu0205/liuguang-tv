package main

import (
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

func reportStartupError(err error) {
	log.Print(err)
	message, e := windows.UTF16PtrFromString(err.Error())
	if e != nil {
		return
	}
	title, _ := windows.UTF16PtrFromString("流光 TV 启动失败")
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
