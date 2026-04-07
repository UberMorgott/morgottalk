//go:build windows

package services

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Win32 API procs for keyboard hook.
// user32 and kern32 are already defined in paste_windows.go.
var (
	pSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	pCallNextHookEx      = user32.NewProc("CallNextHookEx")
	pUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	pGetMessageW         = user32.NewProc("GetMessageW")
	pPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	pGetCurrentThreadId  = kern32.NewProc("GetCurrentThreadId")
)

const (
	whKeyboardLL = 13
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105
	wmQuit       = 0x0012
)

// kbdLLHookStruct matches the Win32 KBDLLHOOKSTRUCT layout.
type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// winMsg matches the Win32 MSG struct layout.
type winMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      [2]int32
}

// hookState holds global state for the single keyboard hook instance.
var hookState struct {
	mu       sync.Mutex
	threadID uint32
	hhook    uintptr
	onKey    func(vk uint16, down bool)
}

// startHook installs a low-level keyboard hook and runs the message pump.
// Blocks until stopHook() is called. Must be called from a goroutine.
// onKey is called from the hook thread for every key event — it must return fast.
// onInstalled is called once after hook installation (nil error = success).
func startHook(onKey func(vk uint16, down bool), onInstalled func(error)) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hookState.mu.Lock()
	hookState.onKey = onKey
	tid, _, _ := pGetCurrentThreadId.Call()
	hookState.threadID = uint32(tid)
	hookState.mu.Unlock()

	// Install the hook
	hhook, _, err := pSetWindowsHookExW.Call(
		whKeyboardLL,
		syscall.NewCallback(llKeyboardProc),
		0, // hInstance = 0 for global hook
		0, // threadId = 0 for global hook
	)
	if hhook == 0 {
		e := fmt.Errorf("SetWindowsHookEx failed: %v", err)
		hookState.mu.Lock()
		hookState.threadID = 0
		hookState.onKey = nil
		hookState.mu.Unlock()
		if onInstalled != nil {
			onInstalled(e)
		}
		return e
	}

	hookState.mu.Lock()
	hookState.hhook = hhook
	hookState.mu.Unlock()

	log.Printf("HotkeyHook: installed (hhook=%#x, tid=%d)", hhook, tid)
	if onInstalled != nil {
		onInstalled(nil)
	}

	// Message pump — required for low-level hooks to receive callbacks
	var m winMsg
	for {
		ret, _, _ := pGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0,
		)
		if ret == 0 || ret == uintptr(^uintptr(0)) { // 0 = WM_QUIT, -1 = error
			break
		}
	}

	// Cleanup
	pUnhookWindowsHookEx.Call(hhook)
	hookState.mu.Lock()
	hookState.hhook = 0
	hookState.threadID = 0
	hookState.onKey = nil
	hookState.mu.Unlock()

	log.Println("HotkeyHook: stopped")
	return nil
}

// stopHook posts WM_QUIT to the hook thread's message pump.
func stopHook() {
	hookState.mu.Lock()
	tid := hookState.threadID
	hookState.mu.Unlock()

	if tid != 0 {
		pPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0)
	}
}

// llKeyboardProc is the Win32 low-level keyboard hook callback.
// Must return quickly (< 200ms) or Windows removes the hook.
func llKeyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && lParam != 0 {
		kb := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
		vk := uint16(kb.VkCode)

		down := wParam == wmKeyDown || wParam == wmSysKeyDown
		up := wParam == wmKeyUp || wParam == wmSysKeyUp

		if down || up {
			hookState.mu.Lock()
			fn := hookState.onKey
			hookState.mu.Unlock()

			if fn != nil {
				fn(vk, down)
			}
		}
	}

	ret, _, _ := pCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}
