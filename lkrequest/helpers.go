package lkrequest

import (
	"fmt"
	"unsafe"
)

const ffiStatusOK int32 = 0

func boolToUintptr(v bool) uintptr {
	if v {
		return 1
	}
	return 0
}

func stringToCString(s string) (uintptr, []byte) {
	buf := append([]byte(s), 0)
	return uintptr(unsafe.Pointer(&buf[0])), buf
}

func optionalStringToCString(s string) (uintptr, []byte) {
	if s == "" {
		return 0, nil
	}
	return stringToCString(s)
}

func bytesPtr(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(unsafe.SliceData(b)))
}

func goStringN(ptr unsafe.Pointer, n uintptr) string {
	if ptr == nil || n == 0 {
		return ""
	}
	buf := make([]byte, int(n))
	copy(buf, unsafe.Slice((*byte)(ptr), int(n)))
	return string(buf)
}

func goCString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}

	const maxLen = 1 << 24 // 16 MiB safety limit
	n := 0
	for n < maxLen && *(*byte)(unsafe.Add(ptr, n)) != 0 {
		n++
	}

	buf := make([]byte, n)
	copy(buf, unsafe.Slice((*byte)(ptr), n))
	return string(buf)
}

func statusError(status int32, action string) error {
	if status == ffiStatusOK {
		return nil
	}
	return fmt.Errorf("lk: %s failed (status=%d)", action, status)
}

func nilHandleError(action string) error {
	return fmt.Errorf("lk: %s returned nil handle", action)
}

// unsupportedError is returned when an FFI symbol is absent from the loaded
// library, i.e. the embedded/linked lkrequest build predates the feature.
func unsupportedError(action string) error {
	return fmt.Errorf("lk: %s is not supported by the loaded lkrequest library", action)
}
