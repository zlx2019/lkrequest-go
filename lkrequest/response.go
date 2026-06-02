package lkrequest

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"unsafe"
)

type Response struct {
	ptr           uintptr
	once          sync.Once
	bodyOnce      sync.Once
	cachedBody    []byte
	UnmarshalJSON func(v any) error
}

func newResponseFromPtr(ptr uintptr) *Response {
	if ptr == 0 {
		return nil
	}

	resp := &Response{ptr: ptr}
	resp.UnmarshalJSON = resp.unmarshalJSON
	runtime.SetFinalizer(resp, (*Response).Close)
	return resp
}

func (r *Response) StatusCode() int {
	if r == nil || r.ptr == 0 {
		return 0
	}
	return int(ffi_lk_response_status(r.ptr))
}

func (r *Response) Version() HttpVersion {
	if r == nil || r.ptr == 0 {
		return HttpVersionUnknown
	}

	var version int32
	if ffi_lk_response_version(r.ptr, uintptr(unsafe.Pointer(&version))) != ffiStatusOK {
		return HttpVersionUnknown
	}

	return HttpVersion(version)
}

func (r *Response) URL() string {
	if r == nil || r.ptr == 0 {
		return ""
	}

	var ptr unsafe.Pointer
	var n uintptr
	if ffi_lk_response_url(r.ptr, uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&n))) != ffiStatusOK {
		return ""
	}

	return goStringN(ptr, n)
}

func (r *Response) HeaderCount() int {
	if r == nil || r.ptr == 0 {
		return 0
	}
	return int(ffi_lk_response_header_count(r.ptr))
}

func (r *Response) HeaderAt(index int) (name, value string) {
	if r == nil || r.ptr == 0 || index < 0 {
		return "", ""
	}

	var namePtr unsafe.Pointer
	var nameLen uintptr
	if ffi_lk_response_header_name_at(
		r.ptr,
		uintptr(index),
		uintptr(unsafe.Pointer(&namePtr)),
		uintptr(unsafe.Pointer(&nameLen)),
	) != ffiStatusOK {
		return "", ""
	}

	var valuePtr unsafe.Pointer
	var valueLen uintptr
	if ffi_lk_response_header_value_at(
		r.ptr,
		uintptr(index),
		uintptr(unsafe.Pointer(&valuePtr)),
		uintptr(unsafe.Pointer(&valueLen)),
	) != ffiStatusOK {
		return "", ""
	}

	return goStringN(namePtr, nameLen), goStringN(valuePtr, valueLen)
}

func (r *Response) Header(name string) string {
	if r == nil || r.ptr == 0 {
		return ""
	}

	var valuePtr unsafe.Pointer
	var valueLen uintptr
	var outErr uintptr

	namePtr, nameBuf := stringToCString(name)
	status := ffi_lk_response_get_header_by_name(
		r.ptr,
		namePtr,
		uintptr(len(name)),
		uintptr(unsafe.Pointer(&valuePtr)),
		uintptr(unsafe.Pointer(&valueLen)),
		uintptr(unsafe.Pointer(&outErr)),
	)
	runtime.KeepAlive(nameBuf)

	if outErr != 0 {
		ffi_lk_error_free(outErr)
	}
	if status != ffiStatusOK {
		return ""
	}

	return goStringN(valuePtr, valueLen)
}

func (r *Response) Headers() http.Header {
	headers := make(http.Header, r.HeaderCount())
	for i := 0; i < r.HeaderCount(); i++ {
		name, value := r.HeaderAt(i)
		if name == "" {
			continue
		}
		headers.Add(name, value)
	}
	return headers
}

func (r *Response) Body() []byte {
	if r == nil || r.ptr == 0 {
		return nil
	}
	r.bodyOnce.Do(func() {
		var bodyPtr unsafe.Pointer
		var bodyLen uintptr
		if ffi_lk_response_body(
			r.ptr,
			uintptr(unsafe.Pointer(&bodyPtr)),
			uintptr(unsafe.Pointer(&bodyLen)),
		) != ffiStatusOK {
			return
		}

		if bodyLen == 0 {
			r.cachedBody = []byte{}
			return
		}

		out := make([]byte, int(bodyLen))
		var readLen uintptr
		if ffi_lk_response_copy_body(
			r.ptr,
			bytesPtr(out),
			uintptr(len(out)),
			uintptr(unsafe.Pointer(&readLen)),
		) == ffiStatusOK {
			r.cachedBody = out[:readLen]
		}
	})
	return r.cachedBody
}

func (r *Response) Bytes() []byte {
	return r.Body()
}

func (r *Response) String() string {
	return string(r.Body())
}

func (r *Response) unmarshalJSON(v any) error {
	return json.Unmarshal(r.Body(), v)
}

func (r *Response) ContentLength() int64 {
	if r == nil || r.ptr == 0 {
		return 0
	}
	return ffi_lk_response_content_length(r.ptr)
}

func (r *Response) DiagnosticsJSON() string {
	if r == nil || r.ptr == 0 {
		return ""
	}

	var ptr unsafe.Pointer
	if ffi_lk_response_get_diagnostics_json(r.ptr, uintptr(unsafe.Pointer(&ptr))) != ffiStatusOK {
		return ""
	}

	return goCString(ptr)
}

func (r *Response) Text() (string, error) {
	if r == nil || r.ptr == 0 {
		return "", ErrNilHandle
	}

	var ptr unsafe.Pointer
	var n uintptr
	var outErr uintptr
	status := ffi_lk_response_text(r.ptr, uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&outErr)))
	if err := extractError(outErr); err != nil {
		return "", err
	}
	if err := statusError(status, "response text"); err != nil {
		return "", err
	}
	return goStringN(ptr, n), nil
}

func (r *Response) ErrorForStatus() error {
	if r == nil || r.ptr == 0 {
		return ErrNilHandle
	}

	var outErr uintptr
	ffi_lk_response_error_for_status(r.ptr, uintptr(unsafe.Pointer(&outErr)))
	return extractError(outErr)
}

func (r *Response) CookieCount() int {
	if r == nil || r.ptr == 0 {
		return 0
	}
	return int(ffi_lk_response_cookie_count(r.ptr))
}

func (r *Response) CookieAt(index int) (name, value string) {
	if r == nil || r.ptr == 0 || index < 0 {
		return "", ""
	}

	var namePtr, valuePtr unsafe.Pointer
	var nameLen, valueLen uintptr
	if ffi_lk_response_cookie_at(r.ptr, uintptr(index), uintptr(unsafe.Pointer(&namePtr)), uintptr(unsafe.Pointer(&nameLen)), uintptr(unsafe.Pointer(&valuePtr)), uintptr(unsafe.Pointer(&valueLen))) != ffiStatusOK {
		return "", ""
	}
	return goStringN(namePtr, nameLen), goStringN(valuePtr, valueLen)
}

// NegotiatedVersion reports the HTTP version actually negotiated for the
// connection (via ALPN / Alt-Svc), which may differ from the version carried
// by the final response. Returns HttpVersionUnknown when the loaded library
// predates this accessor or the version is unavailable.
func (r *Response) NegotiatedVersion() HttpVersion {
	if r == nil || r.ptr == 0 || ffi_lk_response_negotiated_version == nil {
		return HttpVersionUnknown
	}

	var version int32
	if ffi_lk_response_negotiated_version(r.ptr, uintptr(unsafe.Pointer(&version))) != ffiStatusOK {
		return HttpVersionUnknown
	}
	return HttpVersion(version)
}

// CookieNameAt returns the name of the response Set-Cookie at index, or "".
// It is the single-field counterpart to CookieAt.
func (r *Response) CookieNameAt(index int) string {
	if r == nil || r.ptr == 0 || index < 0 || ffi_lk_response_cookie_name_at == nil {
		return ""
	}

	var ptr unsafe.Pointer
	var n uintptr
	if ffi_lk_response_cookie_name_at(r.ptr, uintptr(index), uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&n))) != ffiStatusOK {
		return ""
	}
	return goStringN(ptr, n)
}

// CookieValueAt returns the value of the response Set-Cookie at index, or "".
// It is the single-field counterpart to CookieAt.
func (r *Response) CookieValueAt(index int) string {
	if r == nil || r.ptr == 0 || index < 0 || ffi_lk_response_cookie_value_at == nil {
		return ""
	}

	var ptr unsafe.Pointer
	var n uintptr
	if ffi_lk_response_cookie_value_at(r.ptr, uintptr(index), uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&n))) != ffiStatusOK {
		return ""
	}
	return goStringN(ptr, n)
}

func (r *Response) WasRedirected() bool {
	if r == nil || r.ptr == 0 {
		return false
	}
	return ffi_lk_response_was_redirected(r.ptr) != 0
}

func (r *Response) RedirectCount() int {
	if r == nil || r.ptr == 0 {
		return 0
	}
	return int(ffi_lk_response_redirect_count(r.ptr))
}

func (r *Response) RedirectAt(index int) (url string, status int) {
	if r == nil || r.ptr == 0 || index < 0 {
		return "", 0
	}

	var urlPtr unsafe.Pointer
	var urlLen uintptr
	var statusCode uint16
	if ffi_lk_response_redirect_at(r.ptr, uintptr(index), uintptr(unsafe.Pointer(&urlPtr)), uintptr(unsafe.Pointer(&urlLen)), uintptr(unsafe.Pointer(&statusCode))) != ffiStatusOK {
		return "", 0
	}
	return goStringN(urlPtr, urlLen), int(statusCode)
}

// RedirectURLAt returns the URL of the redirect hop at index, or "".
// It is the single-field counterpart to RedirectAt.
func (r *Response) RedirectURLAt(index int) string {
	if r == nil || r.ptr == 0 || index < 0 || ffi_lk_response_redirect_url_at == nil {
		return ""
	}

	var ptr unsafe.Pointer
	var n uintptr
	if ffi_lk_response_redirect_url_at(r.ptr, uintptr(index), uintptr(unsafe.Pointer(&ptr)), uintptr(unsafe.Pointer(&n))) != ffiStatusOK {
		return ""
	}
	return goStringN(ptr, n)
}

// RedirectStatusAt returns the HTTP status code of the redirect hop at index,
// or 0. It is the single-field counterpart to RedirectAt.
func (r *Response) RedirectStatusAt(index int) int {
	if r == nil || r.ptr == 0 || index < 0 || ffi_lk_response_redirect_status_at == nil {
		return 0
	}
	return int(ffi_lk_response_redirect_status_at(r.ptr, uintptr(index)))
}

func (r *Response) Close() {
	if r == nil {
		return
	}

	r.once.Do(func() {
		ptr := r.ptr
		r.ptr = 0
		r.cachedBody = nil
		runtime.SetFinalizer(r, nil)
		if ptr != 0 {
			ffi_lk_response_free(ptr)
		}
	})
}
