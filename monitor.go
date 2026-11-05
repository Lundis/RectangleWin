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

package main

import (
	"fmt"
	"syscall"

	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
)

func EnumMonitors(f func(d w32.HMONITOR) bool) bool {
	callback := syscall.NewCallback(func(h, _, _, _ uintptr) uintptr {
		if f(w32.HMONITOR(h)) {
			return 1
		}
		return 0
	})
	return w32.EnumDisplayMonitors(0, nil, callback, 0)
}

func printMonitors() {
	i := 0
	EnumMonitors(func(d w32.HMONITOR) bool {
		var v w32.MONITORINFO
		if !w32.GetMonitorInfo(d, &v) {
			return false
		}
		fmt.Printf("> monitor#%d: 0x%x\n", i, d)
		i++
		fmt.Printf("       rcwork:%#v (w=%v,h=%v)\n", v.RcWork, v.RcWork.Width(), v.RcWork.Height())
		fmt.Printf("    rcmonitor:%#v (w=%v,h=%v)\n", v.RcMonitor, v.RcMonitor.Width(), v.RcMonitor.Height())
		fmt.Printf("      primary:%#v\n", v.DwFlags&w32.MONITORINFOF_PRIMARY > 0)

		ok, n := w32.GetNumberOfPhysicalMonitorsFromHMONITOR(d)
		if !ok {
			fmt.Printf("  physical monitors: failed to query count: %d\n", w32.GetLastError())
		} else {
			fmt.Printf("  physical monitors: %d\n", n)
			pMon := make([]w32.PHYSICAL_MONITOR, n)
			if !w32.GetPhysicalMonitorsFromHMONITOR(d, pMon) {
				fmt.Printf("  physical monitors: failed to get physical monitors: %d\n", w32.GetLastError())
			} else {
				for i, p := range pMon {
					name := windows.UTF16ToString(p.Description[:])
					fmt.Printf("  > physical monitor#%d: %s\n", i, name)
				}
			}
		}
		return true
	})
}

type direction int

const (
	dirUp direction = iota
	dirDown
	dirLeft
	dirRight
)

type monitor struct {
	handle w32.HMONITOR
	bounds w32.RECT // full monitor rect
	work   w32.RECT // excludes taskbar
}

func listMonitors() []monitor {
	var out []monitor
	EnumMonitors(func(d w32.HMONITOR) bool {
		var v w32.MONITORINFO
		if w32.GetMonitorInfo(d, &v) {
			out = append(out, monitor{handle: d, bounds: v.RcMonitor, work: v.RcWork})
		}
		return true
	})
	return out
}

func overlap(a1, a2, b1, b2 int32) int32 {
	lo, hi := a1, a2
	if b1 > lo {
		lo = b1
	}
	if b2 < hi {
		hi = b2
	}
	return hi - lo
}

// neighborMonitor returns the monitor adjacent to cur in direction dir. Only
// monitors that are entirely on that side and share some span on the
// perpendicular axis count, so a monitor stacked on top is never picked for
// left/right. Among candidates the closest wins, then the largest shared span.
func neighborMonitor(cur w32.RECT, all []monitor, dir direction) (monitor, bool) {
	var best monitor
	found := false
	var bestGap, bestOverlap int32
	for _, m := range all {
		b := m.bounds
		var gap, ov int32
		switch dir {
		case dirUp:
			gap, ov = cur.Top-b.Bottom, overlap(cur.Left, cur.Right, b.Left, b.Right)
		case dirDown:
			gap, ov = b.Top-cur.Bottom, overlap(cur.Left, cur.Right, b.Left, b.Right)
		case dirLeft:
			gap, ov = cur.Left-b.Right, overlap(cur.Top, cur.Bottom, b.Top, b.Bottom)
		case dirRight:
			gap, ov = b.Left-cur.Right, overlap(cur.Top, cur.Bottom, b.Top, b.Bottom)
		}
		if gap < 0 || ov <= 0 {
			continue
		}
		if !found || gap < bestGap || (gap == bestGap && ov > bestOverlap) {
			best, bestGap, bestOverlap, found = m, gap, ov, true
		}
	}
	return best, found
}

// mapRect maps r from the src area to the same relative position/size in dst.
func mapRect(r, src, dst w32.RECT) w32.RECT {
	sx := func(v int32) int32 {
		return dst.Left + int32(int64(v-src.Left)*int64(dst.Width())/int64(src.Width()))
	}
	sy := func(v int32) int32 {
		return dst.Top + int32(int64(v-src.Top)*int64(dst.Height())/int64(src.Height()))
	}
	out := w32.RECT{Left: sx(r.Left), Top: sy(r.Top), Right: sx(r.Right), Bottom: sy(r.Bottom)}
	return clampRect(out, dst)
}

// clampRect keeps r inside area, shrinking it if it is larger.
func clampRect(r, area w32.RECT) w32.RECT {
	w, h := r.Width(), r.Height()
	if w > area.Width() {
		w = area.Width()
	}
	if h > area.Height() {
		h = area.Height()
	}
	if r.Left < area.Left {
		r.Left = area.Left
	}
	if r.Top < area.Top {
		r.Top = area.Top
	}
	if r.Left+w > area.Right {
		r.Left = area.Right - w
	}
	if r.Top+h > area.Bottom {
		r.Top = area.Bottom - h
	}
	return w32.RECT{Left: r.Left, Top: r.Top, Right: r.Left + w, Bottom: r.Top + h}
}
