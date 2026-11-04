//go:build windows

package turso_libs

import (
	"fmt"
	"syscall"
	"unsafe"
)

// loadLibrary opens the payload at the given absolute verified path via
// LoadLibraryW. Only absolute paths produced by the verified extraction are
// ever passed here.
func loadLibrary(path string) (uintptr, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	loadLibraryW := kernel32.NewProc("LoadLibraryW")
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("invalid payload path: %w", err)
	}
	handle, _, callErr := loadLibraryW.Call(uintptr(unsafe.Pointer(ptr)))
	const errSuccess = uintptr(0)
	if handle == errSuccess {
		return 0, fmt.Errorf("LoadLibraryW failed: %v", callErr)
	}
	return handle, nil
}

// freeLibrary releases a library handle opened by loadLibrary (refcounted).
func freeLibrary(handle uintptr) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	freeLibraryProc := kernel32.NewProc("FreeLibrary")
	r, _, _ := freeLibraryProc.Call(handle)
	if r == 0 {
		return fmt.Errorf("FreeLibrary failed")
	}
	return nil
}
