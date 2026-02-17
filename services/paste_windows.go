//go:build windows

package services

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	kern32 = syscall.NewLazyDLL("kernel32.dll")

	pOpenClipboard    = user32.NewProc("OpenClipboard")
	pCloseClipboard   = user32.NewProc("CloseClipboard")
	pEmptyClipboard   = user32.NewProc("EmptyClipboard")
	pSetClipboardData = user32.NewProc("SetClipboardData")
	pGetClipboardData = user32.NewProc("GetClipboardData")
	pSendInput        = user32.NewProc("SendInput")

	pGlobalAlloc  = kern32.NewProc("GlobalAlloc")
	pGlobalLock   = kern32.NewProc("GlobalLock")
	pGlobalUnlock = kern32.NewProc("GlobalUnlock")
	pGlobalSize   = kern32.NewProc("GlobalSize")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// winClipWrite writes UTF-16 text to the Windows clipboard via Win32 API.
func winClipWrite(text string) error {
	utf16, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("UTF16 conversion: %w", err)
	}

	size := len(utf16) * 2
	hMem, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(size))
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}

	ptr, _, _ := pGlobalLock.Call(hMem)
	if ptr == 0 {
		return fmt.Errorf("GlobalLock failed")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16)), utf16)
	pGlobalUnlock.Call(hMem)

	r, _, _ := pOpenClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("OpenClipboard failed")
	}
	defer pCloseClipboard.Call()

	pEmptyClipboard.Call()
	r, _, _ = pSetClipboardData.Call(cfUnicodeText, hMem)
	if r == 0 {
		return fmt.Errorf("SetClipboardData failed")
	}
	// System owns the memory after SetClipboardData succeeds
	return nil
}

// winClipRead reads UTF-16 text from the Windows clipboard via Win32 API.
func winClipRead() (string, bool) {
	r, _, _ := pOpenClipboard.Call(0)
	if r == 0 {
		return "", false
	}
	defer pCloseClipboard.Call()

	hData, _, _ := pGetClipboardData.Call(cfUnicodeText)
	if hData == 0 {
		return "", false
	}

	ptr, _, _ := pGlobalLock.Call(hData)
	if ptr == 0 {
		return "", false
	}
	defer pGlobalUnlock.Call(hData)

	n, _, _ := pGlobalSize.Call(hData)
	if n == 0 {
		return "", false
	}

	text := syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), n/2))
	return text, true
}

// keyInput matches the C INPUT struct layout for keyboard events on x64.
// Total size: 40 bytes (matches sizeof(INPUT) on 64-bit Windows).
type keyInput struct {
	inputType uint32  // INPUT_KEYBOARD = 1
	_         uint32  // alignment padding
	wVk       uint16  // virtual key code
	wScan     uint16  // scan code
	dwFlags   uint32  // KEYEVENTF_KEYUP etc.
	time      uint32  // timestamp (0 = system)
	dwExtra   uintptr // extra info
	_pad      [8]byte // pad union to MOUSEINPUT size
}

// winSendCtrlV simulates Ctrl+V using SendInput (kernel-level, instant).
func winSendCtrlV() error {
	const (
		inputKeyboard = 1
		keyUp         = 0x0002
		vkControl     = 0x11
		vkV           = 0x56
	)

	inputs := [4]keyInput{
		{inputType: inputKeyboard, wVk: vkControl},
		{inputType: inputKeyboard, wVk: vkV},
		{inputType: inputKeyboard, wVk: vkV, dwFlags: keyUp},
		{inputType: inputKeyboard, wVk: vkControl, dwFlags: keyUp},
	}

	ret, _, err := pSendInput.Call(
		4,
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)
	if ret != 4 {
		return fmt.Errorf("SendInput: %v", err)
	}
	return nil
}
