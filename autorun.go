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
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	AutoRunName = `RectangleWin`
	regKey      = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
)

// self returns the quoted absolute path of the running executable, suitable
// for the Run registry key.
func self() string {
	p, err := os.Executable()
	if err != nil {
		p = os.Args[0]
	}
	return `"` + p + `"`
}

func AutoRunEnabled() (bool, error) {
	rk, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer rk.Close()

	v, _, err := rk.GetStringValue(AutoRunName)
	if errors.Is(err, registry.ErrNotExist) || errors.Is(err, registry.ErrUnexpectedType) {
		return false, nil
	}
	return v == self(), err
}

func AutoRunDisable() error {
	rk, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.WRITE)
	if err != nil {
		return err
	}
	defer rk.Close()

	err = rk.DeleteValue(AutoRunName)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

func AutoRunEnable() error {
	rk, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.WRITE)
	if err != nil {
		return err
	}
	defer rk.Close()
	return rk.SetStringValue(AutoRunName, self())
}
