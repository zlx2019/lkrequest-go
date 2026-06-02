package lkrequest

import "runtime"

func InitLog(level string, filePath string) error {
	levelPtr, levelBuf := stringToCString(level)
	filePathPtr, filePathBuf := stringToCString(filePath)

	status := ffi_lk_log_init(levelPtr, filePathPtr)
	runtime.KeepAlive(levelBuf)
	runtime.KeepAlive(filePathBuf)

	return statusError(status, "log init")
}
