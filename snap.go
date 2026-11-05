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

import "github.com/gonutz/w32/v2"

// TODO find a way to round up divisions consistently, otherwise we end up with off by 1px

func toLeft(d w32.RECT, mul, div int32) w32.RECT {
	return w32.RECT{
		Left:   d.Left,
		Top:    d.Top,
		Right:  d.Left + (d.Width()*mul)/div,
		Bottom: d.Top + d.Height()}
}

func toRight(d w32.RECT, mul, div int32) w32.RECT {
	return w32.RECT{
		Left:   d.Left + d.Width() - d.Width()*mul/div,
		Top:    d.Top,
		Right:  d.Left + d.Width(),
		Bottom: d.Top + d.Height()}
}

func leftHalf(disp, _ w32.RECT) w32.RECT  { return toLeft(disp, 1, 2) }
func rightHalf(disp, _ w32.RECT) w32.RECT { return toRight(disp, 1, 2) }
