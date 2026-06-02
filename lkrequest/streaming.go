package lkrequest

import (
	"io"
	"net/http"
	"runtime"
	"sync"
	"unsafe"
)

type StreamingResponse struct {
	ptr    uintptr
	buf    []byte
	offset int
	done   bool
	once   sync.Once
}

type chunkView struct {
	data    uintptr
	len     uintptr
	isFinal bool
}

func newStreamingResponseFromPtr(ptr uintptr) *StreamingResponse {
	if ptr == 0 {
		return nil
	}

	stream := &StreamingResponse{ptr: ptr}
	runtime.SetFinalizer(stream, (*StreamingResponse).finalize)
	return stream
}

func (s *StreamingResponse) finalize() {
	_ = s.Close()
}

func (s *StreamingResponse) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if s == nil || s.ptr == 0 {
		return 0, io.EOF
	}

	if s.offset < len(s.buf) {
		n := copy(p, s.buf[s.offset:])
		s.offset += n
		if s.offset >= len(s.buf) {
			s.buf = nil
			s.offset = 0
		}
		return n, nil
	}

	if s.done {
		return 0, io.EOF
	}

	for {
		var chunk chunkView
		var outErr uintptr

		status := ffi_lk_stream_read(
			s.ptr,
			uintptr(unsafe.Pointer(&chunk)),
			uintptr(unsafe.Pointer(&outErr)),
		)
		if err := extractError(outErr); err != nil {
			return 0, err
		}
		if err := statusError(status, "stream read"); err != nil {
			return 0, err
		}

		if chunk.len == 0 {
			if chunk.isFinal {
				s.done = true
				return 0, io.EOF
			}
			continue
		}

		s.buf = make([]byte, int(chunk.len))
		s.offset = 0

		var readLen uintptr
		copyStatus := ffi_lk_stream_copy_chunk(
			s.ptr,
			bytesPtr(s.buf),
			uintptr(len(s.buf)),
			uintptr(unsafe.Pointer(&readLen)),
		)
		if err := statusError(copyStatus, "stream copy chunk"); err != nil {
			s.buf = nil
			return 0, err
		}

		s.buf = s.buf[:readLen]
		s.done = chunk.isFinal

		n := copy(p, s.buf)
		s.offset = n
		if s.offset >= len(s.buf) {
			s.buf = nil
			s.offset = 0
		}
		if n > 0 {
			return n, nil
		}
		if s.done {
			return 0, io.EOF
		}
	}
}

func (s *StreamingResponse) Close() error {
	if s == nil {
		return nil
	}

	var closeErr error
	s.once.Do(func() {
		ptr := s.ptr
		s.ptr = 0
		s.buf = nil
		s.offset = 0
		s.done = true
		runtime.SetFinalizer(s, nil)

		if ptr == 0 {
			return
		}

		if err := statusError(ffi_lk_stream_close(ptr), "stream close"); err != nil {
			closeErr = err
		}
		ffi_lk_streaming_response_free(ptr)
	})

	return closeErr
}

func (s *StreamingResponse) StatusCode() int {
	if s == nil || s.ptr == 0 {
		return 0
	}
	return int(ffi_lk_streaming_response_status(s.ptr))
}

func (s *StreamingResponse) HeaderCount() int {
	if s == nil || s.ptr == 0 {
		return 0
	}
	return int(ffi_lk_streaming_response_header_count(s.ptr))
}

func (s *StreamingResponse) HeaderAt(index int) (name, value string) {
	if s == nil || s.ptr == 0 || index < 0 {
		return "", ""
	}

	var namePtr unsafe.Pointer
	var nameLen uintptr
	if ffi_lk_streaming_response_header_name_at(
		s.ptr,
		uintptr(index),
		uintptr(unsafe.Pointer(&namePtr)),
		uintptr(unsafe.Pointer(&nameLen)),
	) != ffiStatusOK {
		return "", ""
	}

	var valuePtr unsafe.Pointer
	var valueLen uintptr
	if ffi_lk_streaming_response_header_value_at(
		s.ptr,
		uintptr(index),
		uintptr(unsafe.Pointer(&valuePtr)),
		uintptr(unsafe.Pointer(&valueLen)),
	) != ffiStatusOK {
		return "", ""
	}

	return goStringN(namePtr, nameLen), goStringN(valuePtr, valueLen)
}

func (s *StreamingResponse) Headers() http.Header {
	headers := make(http.Header, s.HeaderCount())
	for i := 0; i < s.HeaderCount(); i++ {
		name, value := s.HeaderAt(i)
		if name == "" {
			continue
		}
		headers.Add(name, value)
	}
	return headers
}

func (s *StreamingResponse) DiagnosticsJSON() string {
	if s == nil || s.ptr == 0 {
		return ""
	}

	var ptr unsafe.Pointer
	if ffi_lk_streaming_response_get_diagnostics_json(s.ptr, uintptr(unsafe.Pointer(&ptr))) != ffiStatusOK {
		return ""
	}
	return goCString(ptr)
}

func (s *StreamingResponse) Header(name string) string {
	if s == nil || s.ptr == 0 {
		return ""
	}

	var valuePtr unsafe.Pointer
	var valueLen uintptr
	var outErr uintptr
	namePtr, nameBuf := stringToCString(name)
	status := ffi_lk_streaming_response_get_header_by_name(s.ptr, namePtr, uintptr(len(name)), uintptr(unsafe.Pointer(&valuePtr)), uintptr(unsafe.Pointer(&valueLen)), uintptr(unsafe.Pointer(&outErr)))
	runtime.KeepAlive(nameBuf)
	if outErr != 0 {
		ffi_lk_error_free(outErr)
	}
	if status != ffiStatusOK {
		return ""
	}
	return goStringN(valuePtr, valueLen)
}
