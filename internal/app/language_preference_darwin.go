package app

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

// MyGo stores the override in this application's AppleLanguages domain,
// not the global system domain. Read the same CFPreferences value without
// introducing a second persistent language preference.
func savedLanguagePreference(_ string) (string, error) {
	lib, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	if err != nil {
		return "", err
	}
	defer purego.Dlclose(lib)
	var copyMemory func(*uintptr, uintptr, uintptr) uintptr
	purego.RegisterLibFunc(&copyMemory, lib, "memcpy")
	constant := func(name string) (uintptr, error) {
		address, err := purego.Dlsym(lib, name)
		if err != nil {
			return 0, err
		}
		// Dlsym returns the address of an exported CFStringRef variable.
		// Copy its pointer value from native memory rather than constructing
		// a Go pointer into memory managed by CoreFoundation.
		var value uintptr
		copyMemory(&value, address, unsafe.Sizeof(value))
		return value, nil
	}
	app, err := constant("kCFPreferencesCurrentApplication")
	if err != nil {
		return "", err
	}
	user, err := constant("kCFPreferencesCurrentUser")
	if err != nil {
		return "", err
	}
	host, err := constant("kCFPreferencesAnyHost")
	if err != nil {
		return "", err
	}
	var makeString func(uintptr, string, uint32) uintptr
	var release func(uintptr)
	var synchronize func(uintptr, uintptr, uintptr) bool
	var copyValue func(uintptr, uintptr, uintptr, uintptr) uintptr
	var arrayTypeID, stringTypeID func() uintptr
	var getTypeID func(uintptr) uintptr
	var count func(uintptr) int64
	var item func(uintptr, int64) uintptr
	var getString func(uintptr, *byte, int64, uint32) bool
	purego.RegisterLibFunc(&makeString, lib, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&release, lib, "CFRelease")
	purego.RegisterLibFunc(&synchronize, lib, "CFPreferencesSynchronize")
	purego.RegisterLibFunc(&copyValue, lib, "CFPreferencesCopyValue")
	purego.RegisterLibFunc(&getTypeID, lib, "CFGetTypeID")
	purego.RegisterLibFunc(&arrayTypeID, lib, "CFArrayGetTypeID")
	purego.RegisterLibFunc(&stringTypeID, lib, "CFStringGetTypeID")
	purego.RegisterLibFunc(&count, lib, "CFArrayGetCount")
	purego.RegisterLibFunc(&item, lib, "CFArrayGetValueAtIndex")
	purego.RegisterLibFunc(&getString, lib, "CFStringGetCString")
	const utf8 = 0x08000100
	key := makeString(0, "AppleLanguages", utf8)
	if key == 0 {
		return "", fmt.Errorf("cannot create AppleLanguages preference key")
	}
	defer release(key)
	synchronize(app, user, host)
	value := copyValue(key, app, user, host)
	if value == 0 {
		return "", nil
	}
	defer release(value)
	if getTypeID(value) != arrayTypeID() {
		return "", fmt.Errorf("AppleLanguages preference is not an array")
	}
	if count(value) == 0 {
		return "", nil
	}
	first := item(value, 0)
	if getTypeID(first) != stringTypeID() {
		return "", fmt.Errorf("AppleLanguages preference is not a string array")
	}
	// A BCP 47 language tag fits in 255 bytes. Do not read arbitrary-sized
	// preference data into application memory.
	buffer := make([]byte, 256)
	if !getString(first, &buffer[0], int64(len(buffer)), utf8) {
		return "", fmt.Errorf("AppleLanguages preference is not a valid language tag")
	}
	return strings.TrimRight(string(buffer), "\x00"), nil
}
