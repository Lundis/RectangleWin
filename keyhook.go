package main

import (
	"fmt"
	"unsafe"

	"github.com/gonutz/w32/v2"

	"github.com/ahmetb/RectangleWin/w32ex"
)

// Windows reserves Win+Arrow (Snap) for itself, so RegisterHotKey cannot claim
// those combinations. Instead we install a low-level keyboard hook, swallow
// the combinations we handle and post a WM_HOTKEY to our own message loop so
// the actual work happens outside the hook (hooks must return quickly).

const (
	llkhfInjected = 0x10
	// An unassigned virtual key. Tapping it while Win is held stops the
	// Start menu from opening when Win is released (same trick AutoHotkey uses).
	vkMask = 0xE8
)

var (
	hookHandle  w32.HHOOK
	hookThread  uint32
	hookKeys    = map[uint32]int{} // vk -> hotkey id (Win + vk, no other modifiers)
	swallowedDn = map[uint32]bool{}
)

// RegisterWinKeyHook binds Win+vk (with no Ctrl/Alt/Shift) to the callback.
func RegisterWinKeyHook(id int, vk uint32, callback func()) {
	if _, ok := hotkeyRegistrations[id]; ok {
		panic("hotkey id already registered")
	}
	hotkeyRegistrations[id] = &HotKey{id: id, mod: MOD_WIN, vk: int(vk), callback: callback}
	hookKeys[vk] = id
}

// InstallKeyHook must be called from the thread that runs msgLoop.
func InstallKeyHook() error {
	hookThread = w32ex.GetCurrentThreadId()
	hookHandle = w32.SetWindowsHookEx(w32.WH_KEYBOARD_LL, keyHookProc, w32.GetModuleHandle(""), 0)
	if hookHandle == 0 {
		return fmt.Errorf("SetWindowsHookEx failed: %d", w32.GetLastError())
	}
	return nil
}

func isDown(vk int) bool { return w32.GetAsyncKeyState(vk)&0x8000 != 0 }

func keyHookProc(nCode int, wParam w32.WPARAM, lParam w32.LPARAM) w32.LRESULT {
	if nCode < 0 {
		return w32.CallNextHookEx(hookHandle, nCode, wParam, lParam)
	}
	kb := (*w32.KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
	if kb.Flags&llkhfInjected != 0 {
		return w32.CallNextHookEx(hookHandle, nCode, wParam, lParam)
	}
	vk := uint32(kb.VkCode)
	id, handled := hookKeys[vk]
	if !handled {
		return w32.CallNextHookEx(hookHandle, nCode, wParam, lParam)
	}

	switch wParam {
	case w32.WM_KEYDOWN, w32.WM_SYSKEYDOWN:
		winDown := isDown(w32.VK_LWIN) || isDown(w32.VK_RWIN)
		otherMods := isDown(w32.VK_SHIFT) || isDown(w32.VK_CONTROL) || isDown(w32.VK_MENU)
		if !winDown || otherMods {
			break
		}
		if !swallowedDn[vk] { // ignore auto-repeat
			swallowedDn[vk] = true
			suppressStartMenu()
			w32ex.PostThreadMessage(hookThread, w32.WM_HOTKEY, uintptr(id), 0)
		}
		return 1
	case w32.WM_KEYUP, w32.WM_SYSKEYUP:
		if swallowedDn[vk] {
			delete(swallowedDn, vk)
			return 1
		}
	}
	return w32.CallNextHookEx(hookHandle, nCode, wParam, lParam)
}

func suppressStartMenu() {
	w32.SendInput(
		w32.KeyboardInput(w32.KEYBDINPUT{Vk: vkMask}),
		w32.KeyboardInput(w32.KEYBDINPUT{Vk: vkMask, Flags: w32.KEYEVENTF_KEYUP}),
	)
}
