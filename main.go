// Copyright 2022 Ahmet Alp Balkan
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// TODO make it possible to "go generate" on Windows (https://github.com/josephspurrier/goversioninfo/issues/52).
//go:generate /bin/bash -c "go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest -arm -64 -icon=assets/icon.ico - <<< '{}'"

package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"

	"github.com/getlantern/systray"
	"github.com/gonutz/w32/v2"

	"github.com/ahmetb/RectangleWin/w32ex"
)

// perMonitorDPI is true when the process runs as per-monitor (v2) DPI aware,
// in which case every coordinate we deal with is in physical pixels.
var perMonitorDPI bool

func main() {
	runtime.LockOSThread() // since we bind hotkeys etc that need to dispatch their message here
	perMonitorDPI = w32ex.SetProcessDpiAwarenessPerMonitorV2()
	if !perMonitorDPI && !w32ex.SetProcessDPIAware() {
		panic("failed to set DPI aware")
	}
	fmt.Printf("per-monitor DPI aware=%v\n", perMonitorDPI)

	autorun, err := AutoRunEnabled()
	if err != nil {
		panic(err)
	}
	fmt.Printf("autorun enabled=%v\n", autorun)
	printMonitors()

	// Win+Arrow and Win+Enter are reserved by Windows, so they go through a
	// low-level keyboard hook instead of RegisterHotKey.
	logErr := func(name string, f func() error) func() {
		return func() {
			if err := f(); err != nil {
				fmt.Printf("warn: %s: %v\n", name, err)
			}
		}
	}
	RegisterWinKeyHook(1, w32.VK_LEFT, logErr("snap left", func() error { return snapHalf(dirLeft) }))
	RegisterWinKeyHook(2, w32.VK_RIGHT, logErr("snap right", func() error { return snapHalf(dirRight) }))
	RegisterWinKeyHook(3, w32.VK_UP, logErr("move up", func() error { return moveToMonitor(dirUp) }))
	RegisterWinKeyHook(4, w32.VK_DOWN, logErr("move down", func() error { return moveToMonitor(dirDown) }))
	RegisterWinKeyHook(5, w32.VK_RETURN, logErr("fullscreen", toggleMaximize))
	if err := InstallKeyHook(); err != nil {
		showMessageBox("Failed to install keyboard hook: " + err.Error())
		panic(err)
	}

	hks := []HotKey{
		(HotKey{id: 60, mod: MOD_ALT | MOD_WIN, vk: 0x43 /*C*/, callback: func() {
			if _, err := resize(w32.GetForegroundWindow(), center); err != nil {
				fmt.Printf("warn: resize: %v\n", err)
				return
			}
		}}),
		(HotKey{id: 70, mod: MOD_ALT | MOD_WIN, vk: 0x41 /*A*/, callback: func() {
			hwnd := w32.GetForegroundWindow()
			if err := toggleAlwaysOnTop(hwnd); err != nil {
				fmt.Printf("warn: toggleAlwaysOnTop: %v\n", err)
				return
			}
			fmt.Printf("> toggled always on top: %v\n", hwnd)
		}}),
	}

	var failedHotKeys []HotKey
	for _, hk := range hks {
		if !RegisterHotKey(hk) {
			failedHotKeys = append(failedHotKeys, hk)
		}
	}
	if len(failedHotKeys) > 0 {
		msg := "The following hotkey(s) are in use by another process:\n\n"
		for _, hk := range failedHotKeys {
			msg += "  - " + hk.Describe() + "\n"
		}
		msg += "\nTo use these hotkeys in RectangleWin, close the other process using the key combination(s)."
		showMessageBox(msg)
	}

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, os.Interrupt)
	go func() {
		<-exitCh
		fmt.Println("exit signal received")
		systray.Quit() // causes WM_CLOSE, WM_QUIT, not sure if a side-effect
	}()

	// TODO systray/systray.go already locks the OS thread in init()
	// however it's not clear if GetMessage(0,0) will continue to work
	// as we run "go initTray()" and not pin the thread that initializes the
	// tray.
	initTray()
	if err := msgLoop(); err != nil {
		panic(err)
	}
}

func showMessageBox(text string) {
	w32.MessageBox(w32.GetActiveWindow(), text, "RectangleWin", w32.MB_ICONWARNING|w32.MB_OK)
}

type resizeFunc func(disp, cur w32.RECT) w32.RECT

func center(disp, cur w32.RECT) w32.RECT {
	// TODO find a way to round up divisions consistently as it causes multiple runs to shift by 1px
	w := (disp.Width() - cur.Width()) / 2
	h := (disp.Height() - cur.Height()) / 2
	return w32.RECT{
		Left:   disp.Left + w,
		Right:  disp.Left + w + cur.Width(),
		Top:    disp.Top + h,
		Bottom: disp.Top + h + cur.Height()}
}

func resize(hwnd w32.HWND, f resizeFunc) (bool, error) {
	return resizeOn(hwnd, 0, f)
}

// windowFrame returns the visible frame of the window (excluding the invisible
// resize borders) in the same coordinate space as GetWindowRect.
func windowFrame(hwnd w32.HWND) (w32.RECT, error) {
	ok, frame := w32.DwmGetWindowAttributeEXTENDED_FRAME_BOUNDS(hwnd)
	if !ok {
		return w32.RECT{}, fmt.Errorf("failed to DwmGetWindowAttributeEXTENDED_FRAME_BOUNDS:%d", w32.GetLastError())
	}
	if perMonitorDPI {
		return frame, nil // DWM bounds are physical pixels, same as GetWindowRect
	}
	hdc := w32.GetDC(hwnd)
	displayDPI := w32.GetDeviceCaps(hdc, w32.LOGPIXELSY)
	if !w32.ReleaseDC(hwnd, hdc) {
		return w32.RECT{}, fmt.Errorf("failed to ReleaseDC:%d", w32.GetLastError())
	}
	return resizeForDpi(frame, w32ex.GetDpiForWindow(hwnd), int32(displayDPI)), nil
}

// resizeOn positions hwnd at f(workArea, currentFrame). The work area is that
// of monitor mon, or of the window's current monitor if mon is 0.
func resizeOn(hwnd w32.HWND, mon w32.HMONITOR, f resizeFunc) (bool, error) {
	if !isZonableWindow(hwnd) {
		fmt.Printf("warn: non-zonable window: %s\n", w32.GetWindowText(hwnd))
		return false, nil
	}
	rect := w32.GetWindowRect(hwnd)
	if mon == 0 {
		mon = w32.MonitorFromWindow(hwnd, w32.MONITOR_DEFAULTTONEAREST)
	}
	var monInfo w32.MONITORINFO
	if !w32.GetMonitorInfo(mon, &monInfo) {
		return false, fmt.Errorf("failed to GetMonitorInfo:%d", w32.GetLastError())
	}
	resizedFrame, err := windowFrame(hwnd)
	if err != nil {
		return false, err
	}

	fmt.Printf("> window: 0x%x %#v (w:%d,h:%d) mon=0x%X\n", hwnd, rect, rect.Width(), rect.Height(), mon)
	fmt.Printf("> frame:  %#v (W:%d,H:%d)\n", resizedFrame, resizedFrame.Width(), resizedFrame.Height())

	// calculate how many extra pixels go to win10 invisible borders
	lExtra := resizedFrame.Left - rect.Left
	rExtra := -resizedFrame.Right + rect.Right
	tExtra := resizedFrame.Top - rect.Top
	bExtra := -resizedFrame.Bottom + rect.Bottom

	newPos := f(monInfo.RcWork, resizedFrame)

	// adjust offsets based on invisible borders
	newPos.Left -= lExtra
	newPos.Top -= tExtra
	newPos.Right += rExtra
	newPos.Bottom += bExtra

	if sameRect(rect, &newPos) {
		fmt.Println("no resize")
		return false, nil
	}

	fmt.Printf("> resizing to: %#v (W:%d,H:%d)\n", newPos, newPos.Width(), newPos.Height())
	if !w32.ShowWindow(hwnd, w32.SW_SHOWNORMAL) { // normalize window first if it's set to SW_SHOWMAXIMIZE (and therefore stays maximized)
		return false, fmt.Errorf("failed to normalize window ShowWindow:%d", w32.GetLastError())
	}
	if !w32.SetWindowPos(hwnd, 0, int(newPos.Left), int(newPos.Top), int(newPos.Width()), int(newPos.Height()), w32.SWP_NOZORDER|w32.SWP_NOACTIVATE) {
		return false, fmt.Errorf("failed to SetWindowPos:%d", w32.GetLastError())
	}
	rect = w32.GetWindowRect(hwnd)
	fmt.Printf("> post-resize: %#v(W:%d,H:%d)\n", rect, rect.Width(), rect.Height())
	return true, nil
}

func toggleMaximize() error {
	hwnd := w32.GetForegroundWindow()
	if !isZonableWindow(hwnd) {
		return errors.New("foreground window is not zonable")
	}
	cmd := w32.SW_MAXIMIZE
	if w32ex.IsZoomed(hwnd) {
		cmd = w32.SW_RESTORE
	}
	w32.ShowWindow(hwnd, cmd) // return value is the previous visibility, not success
	return nil
}

// snapHalf puts the window on the left/right half of its monitor. If it is
// already there and a monitor exists on that side (side by side, not stacked),
// it continues onto the near half of that monitor.
func snapHalf(dir direction) error {
	hwnd := w32.GetForegroundWindow()
	if !isZonableWindow(hwnd) {
		return errors.New("foreground window is not zonable")
	}
	half, opposite := resizeFunc(leftHalf), resizeFunc(rightHalf)
	if dir == dirRight {
		half, opposite = rightHalf, leftHalf
	}

	var monInfo w32.MONITORINFO
	if !w32.GetMonitorInfo(w32.MonitorFromWindow(hwnd, w32.MONITOR_DEFAULTTONEAREST), &monInfo) {
		return fmt.Errorf("failed to GetMonitorInfo:%d", w32.GetLastError())
	}
	frame, err := windowFrame(hwnd)
	if err != nil {
		return err
	}
	if !w32ex.IsZoomed(hwnd) && closeRect(frame, half(monInfo.RcWork, frame), 2) {
		if next, ok := neighborMonitor(monInfo.RcMonitor, listMonitors(), dir); ok {
			return placeOnMonitor(hwnd, next.handle, opposite(next.work, frame), false)
		}
		return nil
	}
	_, err = resize(hwnd, half)
	return err
}

// moveToMonitor moves the window to the neighbouring monitor in dir, keeping
// its relative position and size (and maximized state).
func moveToMonitor(dir direction) error {
	hwnd := w32.GetForegroundWindow()
	if !isZonableWindow(hwnd) {
		return errors.New("foreground window is not zonable")
	}
	var monInfo w32.MONITORINFO
	if !w32.GetMonitorInfo(w32.MonitorFromWindow(hwnd, w32.MONITOR_DEFAULTTONEAREST), &monInfo) {
		return fmt.Errorf("failed to GetMonitorInfo:%d", w32.GetLastError())
	}
	next, ok := neighborMonitor(monInfo.RcMonitor, listMonitors(), dir)
	if !ok {
		fmt.Println("no monitor in that direction")
		return nil
	}
	maximized := w32ex.IsZoomed(hwnd)
	target := next.work
	if !maximized {
		frame, err := windowFrame(hwnd)
		if err != nil {
			return err
		}
		target = mapRect(frame, monInfo.RcWork, next.work)
	}
	return placeOnMonitor(hwnd, next.handle, target, maximized)
}

// placeOnMonitor moves the window's visible frame to target on monitor mon.
// It runs twice because a window crossing to a monitor with a different scale
// factor may resize itself in response to WM_DPICHANGED.
func placeOnMonitor(hwnd w32.HWND, mon w32.HMONITOR, target w32.RECT, maximize bool) error {
	to := func(_, _ w32.RECT) w32.RECT { return target }
	for i := 0; i < 2; i++ {
		if _, err := resizeOn(hwnd, mon, to); err != nil {
			return err
		}
	}
	if maximize {
		w32.ShowWindow(hwnd, w32.SW_MAXIMIZE)
	}
	return nil
}

func closeRect(a, b w32.RECT, tolerance int32) bool {
	abs := func(v int32) int32 {
		if v < 0 {
			return -v
		}
		return v
	}
	return abs(a.Left-b.Left) <= tolerance && abs(a.Top-b.Top) <= tolerance &&
		abs(a.Right-b.Right) <= tolerance && abs(a.Bottom-b.Bottom) <= tolerance
}

func toggleAlwaysOnTop(hwnd w32.HWND) error {
	if !isZonableWindow(hwnd) {
		return errors.New("foreground window is not zonable")
	}

	if w32.GetWindowLong(hwnd, w32.GWL_EXSTYLE)&w32.WS_EX_TOPMOST != 0 {
		if !w32.SetWindowPos(hwnd, w32.HWND_NOTOPMOST, 0, 0, 0, 0, w32.SWP_NOMOVE|w32.SWP_NOSIZE) {
			return fmt.Errorf("failed to SetWindowPos(HWND_NOTOPMOST): %v", w32.GetLastError())
		}
	} else {
		if !w32.SetWindowPos(hwnd, w32.HWND_TOPMOST, 0, 0, 0, 0, w32.SWP_NOMOVE|w32.SWP_NOSIZE) {
			return fmt.Errorf("failed to SetWindowPos(HWND_TOPMOST) :%v", w32.GetLastError())
		}
	}
	return nil
}

func resizeForDpi(src w32.RECT, from, to int32) w32.RECT {
	return w32.RECT{
		Left:   src.Left * to / from,
		Right:  src.Right * to / from,
		Top:    src.Top * to / from,
		Bottom: src.Bottom * to / from,
	}
}

func sameRect(a, b *w32.RECT) bool {
	return a != nil && b != nil && reflect.DeepEqual(*a, *b)
}
