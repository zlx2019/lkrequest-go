package lkrequest

import (
	"runtime"
	"unsafe"
)

func ListPresetsJSON() (string, error) {
	var outJSON unsafe.Pointer

	status := ffi_lk_preset_list_json(uintptr(unsafe.Pointer(&outJSON)))
	if err := statusError(status, "preset list json"); err != nil {
		return "", err
	}

	return goCString(outJSON), nil
}

func GetPresetDetailJSON(name string) (string, error) {
	var outJSON unsafe.Pointer
	var outErr uintptr

	namePtr, nameBuf := stringToCString(name)
	status := ffi_lk_preset_get_detail_json(
		namePtr,
		uintptr(len(name)),
		uintptr(unsafe.Pointer(&outJSON)),
		uintptr(unsafe.Pointer(&outErr)),
	)
	runtime.KeepAlive(nameBuf)

	if err := extractError(outErr); err != nil {
		return "", err
	}
	if err := statusError(status, "preset get detail json"); err != nil {
		return "", err
	}
	if outJSON == nil {
		return "", nilHandleError("preset get detail json")
	}

	return goCString(outJSON), nil
}
