//go:build js

package main

import (
	"fmt"
	"io/fs"
	"syscall/js"
)

// inBrowser is whether this is the browser build, which has no files and no window to close: the run is
// saved every turn instead, and there is nothing to quit to.
const inBrowser = true

// hasData, readData, writeData and removeData keep the save and the high scores, by name, in the browser's
// local storage. It can be missing or refuse writes (a private window, a full quota), which only loses them.
// hasData doesn't copy the data out, since the title screen asks every frame.
func hasData(name string) (ok bool) {
	local(func(s js.Value) { ok = !s.Call("getItem", "underwick/"+name).IsNull() })
	return ok
}

func readData(name string) (data []byte, err error) {
	err = local(func(s js.Value) {
		if v := s.Call("getItem", "underwick/"+name); v.IsNull() {
			err = fs.ErrNotExist
		} else {
			data = []byte(v.String())
		}
	})
	return data, err
}

func writeData(name string, data []byte) error {
	return local(func(s js.Value) { s.Call("setItem", "underwick/"+name, string(data)) })
}

func removeData(name string) { local(func(s js.Value) { s.Call("removeItem", "underwick/"+name) }) }

// local runs f on localStorage, turning the JavaScript exception it may throw into an error.
func local(f func(js.Value)) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("local storage: %v", r)
		}
	}()
	s := js.Global().Get("localStorage")
	if s.IsUndefined() || s.IsNull() {
		return fs.ErrNotExist
	}
	f(s)
	return err
}
