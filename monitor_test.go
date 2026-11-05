package main

import (
	"testing"

	"github.com/gonutz/w32/v2"
)

func rect(l, t, r, b int32) w32.RECT { return w32.RECT{Left: l, Top: t, Right: r, Bottom: b} }

func TestNeighborMonitorStacked(t *testing.T) {
	// primary 1920x1080 at origin, a 2560x1440 monitor stacked on top of it
	// (slightly offset horizontally, as Windows display settings often are).
	lower := monitor{handle: 1, bounds: rect(0, 0, 1920, 1080)}
	upper := monitor{handle: 2, bounds: rect(-300, -1440, 2260, 0)}
	all := []monitor{lower, upper}

	cases := []struct {
		name string
		cur  monitor
		dir  direction
		want w32.HMONITOR // 0 = none
	}{
		{"lower up", lower, dirUp, 2},
		{"lower down", lower, dirDown, 0},
		{"lower left", lower, dirLeft, 0},
		{"lower right", lower, dirRight, 0},
		{"upper down", upper, dirDown, 1},
		{"upper up", upper, dirUp, 0},
		{"upper left", upper, dirLeft, 0},
		{"upper right", upper, dirRight, 0},
	}
	for _, c := range cases {
		got, ok := neighborMonitor(c.cur.bounds, all, c.dir)
		if c.want == 0 && ok {
			t.Errorf("%s: expected none, got %v", c.name, got.handle)
		}
		if c.want != 0 && (!ok || got.handle != c.want) {
			t.Errorf("%s: expected %v, got %v (ok=%v)", c.name, c.want, got.handle, ok)
		}
	}
}

func TestNeighborMonitorSideBySide(t *testing.T) {
	a := monitor{handle: 1, bounds: rect(0, 0, 1920, 1080)}
	b := monitor{handle: 2, bounds: rect(1920, 0, 3840, 1080)}
	if got, ok := neighborMonitor(a.bounds, []monitor{a, b}, dirRight); !ok || got.handle != 2 {
		t.Errorf("expected right neighbour")
	}
	if _, ok := neighborMonitor(a.bounds, []monitor{a, b}, dirUp); ok {
		t.Errorf("expected no up neighbour")
	}
}

func TestMapRect(t *testing.T) {
	src := rect(0, 0, 1920, 1040)
	dst := rect(-300, -1440, 2260, -40)
	// left half maps to left half
	got := mapRect(rect(0, 0, 960, 1040), src, dst)
	if want := rect(-300, -1440, 980, -40); got != want {
		t.Errorf("got %+v want %+v", got, want)
	}
	// oversized window is clamped into the target
	got = mapRect(rect(-50, -50, 2000, 1100), src, rect(0, 0, 800, 600))
	if got.Left < 0 || got.Top < 0 || got.Right > 800 || got.Bottom > 600 {
		t.Errorf("not clamped: %+v", got)
	}
}
