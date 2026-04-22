package tray

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include "tray.h"
*/
import "C"
import "unsafe"

var (
	onOpenFn   func()
	onQuitFn   func()
	onToggleFn func()
)

// Init creates the macOS status-bar icon.
// Must be called after the Wails event loop has started (e.g. from app.startup).
func Init(iconPNG []byte, integrationEnabled bool, onOpen, onQuit, onToggle func()) {
	onOpenFn = onOpen
	onQuitFn = onQuit
	onToggleFn = onToggle
	enabled := C.int(0)
	if integrationEnabled {
		enabled = C.int(1)
	}
	C.initTray((*C.char)(unsafe.Pointer(&iconPNG[0])), C.int(len(iconPNG)), enabled)
}

// UpdateToggle updates the checkmark on the tray toggle item.
func UpdateToggle(enabled bool) {
	e := C.int(0)
	if enabled {
		e = C.int(1)
	}
	C.updateTrayToggle(e)
}

//export goTrayOpen
func goTrayOpen() {
	if onOpenFn != nil {
		onOpenFn()
	}
}

//export goTrayQuit
func goTrayQuit() {
	if onQuitFn != nil {
		onQuitFn()
	}
}

//export goTrayToggle
func goTrayToggle() {
	if onToggleFn != nil {
		onToggleFn()
	}
}
