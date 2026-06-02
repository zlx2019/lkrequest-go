package lkrequest

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// LogLevel controls the minimum severity captured by InitLogCallback.
type LogLevel int32

const (
	LogLevelTrace LogLevel = 0
	LogLevelDebug LogLevel = 1
	LogLevelInfo  LogLevel = 2
	LogLevelWarn  LogLevel = 3
	LogLevelError LogLevel = 4
)

// ProxyProvider supplies proxy URLs on demand for ProxyPoolBuilder / SessionPoolBuilder.
// Implementations must return quickly and avoid blocking I/O.
type ProxyProvider interface {
	NextProxy() (url string, ok bool)
	Len() uint
	IsDynamic() bool
}

// ProxyProviderFuncs is a function-based ProxyProvider helper.
type ProxyProviderFuncs struct {
	NextProxyFunc func() (string, bool)
	LenFunc       func() uint
	IsDynamicFunc func() bool
	DestroyFunc   func()
}

func (p ProxyProviderFuncs) NextProxy() (string, bool) {
	if p.NextProxyFunc == nil {
		return "", false
	}
	return p.NextProxyFunc()
}

func (p ProxyProviderFuncs) Len() uint {
	if p.LenFunc == nil {
		return 0
	}
	return p.LenFunc()
}

func (p ProxyProviderFuncs) IsDynamic() bool {
	if p.IsDynamicFunc == nil {
		return false
	}
	return p.IsDynamicFunc()
}

func (p ProxyProviderFuncs) Destroy() {
	if p.DestroyFunc != nil {
		p.DestroyFunc()
	}
}

type proxyProviderDestroyer interface {
	Destroy()
}

type ffiProxyProvider struct {
	context   uintptr
	nextProxy uintptr
	len       uintptr
	isDynamic uintptr
	destroy   uintptr
}

type proxyProviderState struct {
	provider ProxyProvider
	mu       sync.Mutex
	scratch  []byte
	pinner   runtime.Pinner
	pinned   bool
}

func (s *proxyProviderState) ensureScratchCapacity(n int) {
	if n < 4096 {
		n = 4096
	}
	if cap(s.scratch) >= n && s.pinned {
		return
	}
	if s.pinned {
		s.pinner.Unpin()
		s.pinned = false
	}
	s.scratch = make([]byte, n)
	s.pinner.Pin(&s.scratch[0])
	s.pinned = true
}

func (s *proxyProviderState) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pinned {
		s.pinner.Unpin()
		s.pinned = false
	}
	if destroyer, ok := s.provider.(proxyProviderDestroyer); ok {
		destroyer.Destroy()
	}
}

var (
	nextCallbackContextID atomic.Uintptr
	logCallbackRegistry   sync.Map
	providerRegistry      sync.Map

	logCallbackBridgePtr       = purego.NewCallback(logCallbackBridge)
	providerNextProxyBridgePtr = purego.NewCallback(providerNextProxyBridge)
	providerLenBridgePtr       = purego.NewCallback(providerLenBridge)
	providerIsDynamicBridgePtr = purego.NewCallback(providerIsDynamicBridge)
	providerDestroyBridgePtr   = purego.NewCallback(providerDestroyBridge)
)

func nextCallbackContext() uintptr {
	return nextCallbackContextID.Add(1)
}

func InitLogCallback(callback func(level LogLevel, target, message string), minLevel LogLevel) error {
	if callback == nil {
		return fmt.Errorf("lk: log callback is nil")
	}

	contextID := nextCallbackContext()
	logCallbackRegistry.Store(contextID, callback)

	status := ffi_lk_log_init_callback(logCallbackBridgePtr, contextID, int32(minLevel))
	if status != ffiStatusOK {
		logCallbackRegistry.Delete(contextID)
	}
	return statusError(status, "log init callback")
}

func logCallbackBridge(context uintptr, level int32, target uintptr, message uintptr) uintptr {
	value, ok := logCallbackRegistry.Load(context)
	if !ok {
		return 0
	}
	callback, ok := value.(func(level LogLevel, target, message string))
	if !ok || callback == nil {
		return 0
	}
	callback(
		LogLevel(level),
		goCString(unsafe.Pointer(target)),
		goCString(unsafe.Pointer(message)),
	)
	return 0
}

func newFFIProxyProvider(provider ProxyProvider) (ffiProxyProvider, func(), error) {
	if provider == nil {
		return ffiProxyProvider{}, nil, fmt.Errorf("lk: proxy provider is nil")
	}

	contextID := nextCallbackContext()
	state := &proxyProviderState{provider: provider}
	state.ensureScratchCapacity(0)
	providerRegistry.Store(contextID, state)

	release := func() {
		if value, ok := providerRegistry.LoadAndDelete(contextID); ok {
			value.(*proxyProviderState).close()
		}
	}

	return ffiProxyProvider{
		context:   contextID,
		nextProxy: providerNextProxyBridgePtr,
		len:       providerLenBridgePtr,
		isDynamic: providerIsDynamicBridgePtr,
		destroy:   providerDestroyBridgePtr,
	}, release, nil
}

func providerStateFor(context uintptr) *proxyProviderState {
	if context == 0 {
		return nil
	}
	value, ok := providerRegistry.Load(context)
	if !ok {
		return nil
	}
	return value.(*proxyProviderState)
}

func providerNextProxyBridge(context uintptr, outURLPtr uintptr, outURLLen uintptr) uintptr {
	if outURLPtr != 0 {
		*(*uintptr)(unsafe.Pointer(outURLPtr)) = 0
	}
	if outURLLen != 0 {
		*(*uintptr)(unsafe.Pointer(outURLLen)) = 0
	}

	state := providerStateFor(context)
	if state == nil {
		return ^uintptr(0)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	url, ok := state.provider.NextProxy()
	if !ok {
		return ^uintptr(0)
	}
	if len(url) == 0 {
		return 0
	}

	state.ensureScratchCapacity(len(url))
	state.scratch = state.scratch[:len(url)]
	copy(state.scratch, url)

	if outURLPtr != 0 {
		*(*uintptr)(unsafe.Pointer(outURLPtr)) = bytesPtr(state.scratch)
	}
	if outURLLen != 0 {
		*(*uintptr)(unsafe.Pointer(outURLLen)) = uintptr(len(state.scratch))
	}
	return 0
}

func providerLenBridge(context uintptr) uintptr {
	state := providerStateFor(context)
	if state == nil {
		return 0
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	return uintptr(state.provider.Len())
}

func providerIsDynamicBridge(context uintptr) uintptr {
	state := providerStateFor(context)
	if state == nil {
		return 0
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.provider.IsDynamic() {
		return 1
	}
	return 0
}

func providerDestroyBridge(context uintptr) uintptr {
	if value, ok := providerRegistry.LoadAndDelete(context); ok {
		value.(*proxyProviderState).close()
	}
	return 0
}
