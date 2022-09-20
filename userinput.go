package main

import (
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

var (
	DllUser32        = windows.NewLazyDLL("user32.dll")
	EntryGetKeyState = DllUser32.NewProc("GetKeyState")
)

var UserInput UserInputStruct

type CallbackEventType byte

const (
	CALLBACK_EVENT_SYSTEM_KEYDOWN CallbackEventType = iota
	CALLBACK_EVENT_SYSTEM_KEYUP
	CALLBACK_EVENT_KEYDOWN
	CALLBACK_EVENT_KEYUP
	CALLBACK_EVENT_MOUSEBTNCLICK
	CALLBACK_EVENT_MOUSEBTNPUSH
	CALLBACK_EVENT_MOUSEBTNRELEASE
	CALLBACK_EVENT_MOUSEMOTION
	CALLBACK_EVENT_WINDOWLEAVE
)

type CallbackType byte

const (
	CALLBACK_TYPE_KEYBOARD CallbackType = iota
	CALLBACK_TYPE_MOUSE
	CALLBACK_TYPE_WINDOW
)

type CallbackHandle int

type InputCallback func(callbackData InputCallbackDataI)

type InputCallbackDataI interface {
	InputCallbackDataIDump()
}

type WindowCallbackData struct {
	CbEvType CallbackEventType
}

func (cd WindowCallbackData) InputCallbackDataIDump() {}

type KeyboardCallbackData struct {
	CbEvType CallbackEventType
}

func (cd KeyboardCallbackData) InputCallbackDataIDump() {}

type MouseCallbackData struct {
	CbEvType CallbackEventType
	X        int
	Y        int
}

func (cd MouseCallbackData) InputCallbackDataIDump() {}

type handleDataI interface {
	IsFree() bool
	IsNew() bool
	SwitchOffNew()
}

type windowHandleData struct {
	Callback InputCallback
	NewEv    bool
}

func (cd *windowHandleData) IsFree() bool  { return cd.Callback == nil }
func (cd *windowHandleData) IsNew() bool   { return cd.NewEv }
func (cd *windowHandleData) SwitchOffNew() { cd.NewEv = false }

type keyboardHandleData struct {
	Key      byte
	State    byte
	Callback InputCallback
	NewEv    bool
	SystemEv bool
}

func (cd *keyboardHandleData) IsFree() bool  { return cd.Callback == nil }
func (cd *keyboardHandleData) IsNew() bool   { return cd.NewEv }
func (cd *keyboardHandleData) SwitchOffNew() { cd.NewEv = false }

type mouseHandleData struct {
	Btn      byte
	State    byte
	Callback InputCallback
	NewEv    bool
}

func (cd *mouseHandleData) IsFree() bool  { return cd.Callback == nil }
func (cd *mouseHandleData) IsNew() bool   { return cd.NewEv }
func (cd *mouseHandleData) SwitchOffNew() { cd.NewEv = false }

type UserInputStruct struct {
	mut      sync.Mutex
	isActive bool
	stop     chan int
	handlers map[CallbackHandle]handleDataI
}

func (ui *UserInputStruct) StartUpdate() {
	worker := func(stop chan int) {
		ticker := time.NewTicker(time.Millisecond)
		for {
			select {
			case <-stop:
				ticker.Stop()
				return
			case <-ticker.C:
				UserInput.update()
			}
		}
	}
	if !ui.isActive {
		ui.stop = make(chan int)
		ui.handlers = make(map[CallbackHandle]handleDataI)
		go worker(ui.stop)
		ui.isActive = true
	}
}

func (ui *UserInputStruct) StopUpdate() {
	if ui.isActive {
		ui.stop <- 0
		close(ui.stop)
		ui.isActive = false
	}
}

func gkstate(key byte) byte {
	x := GetKeyState(key)
	return byte(x >> 8)
}

func (ui *UserInputStruct) update() {
	ui.mut.Lock()
	for _, handle := range ui.handlers {
		if !handle.IsFree() && !handle.IsNew() {
			switch t := handle.(type) {
			case *keyboardHandleData:
				if t.SystemEv {
					s := gkstate(t.Key)
					if s != t.State {
						t.State = s
						if s != 0 {
							t.Callback(&KeyboardCallbackData{
								CbEvType: CALLBACK_EVENT_SYSTEM_KEYDOWN,
							})
						} else {
							t.Callback(&KeyboardCallbackData{
								CbEvType: CALLBACK_EVENT_SYSTEM_KEYUP,
							})
						}
					}
				}
			}
		}
	}

	if len(ui.handlers) > 10 {
		if ui.handlers[CallbackHandle(len(ui.handlers)-1)].IsFree() {
			delete(ui.handlers, CallbackHandle(len(ui.handlers)-1))
		}
	}

	for _, v := range ui.handlers {
		if v.IsNew() {
			v.SwitchOffNew()
		}
	}

	ui.mut.Unlock()
}

func (ui *UserInputStruct) KeyboardUpdate(key byte, state byte) {
	ui.mut.Lock()
	for _, handle := range ui.handlers {
		if !handle.IsFree() && !handle.IsNew() {
			switch t := handle.(type) {
			case *keyboardHandleData:
				if !t.SystemEv {
					if t.Key == key {
						if state != t.State {
							t.State = state
							if state != 0 {
								t.Callback(&KeyboardCallbackData{
									CbEvType: CALLBACK_EVENT_KEYDOWN,
								})
							} else {
								t.Callback(&KeyboardCallbackData{
									CbEvType: CALLBACK_EVENT_KEYUP,
								})
							}
						}
					}
				}
			}
		}
	}
	ui.mut.Unlock()
}

func (ui *UserInputStruct) MouseBtnUpdate(btn byte, s uint32, x int32, y int32) {
	ui.mut.Lock()
	for _, handle := range ui.handlers {
		if !handle.IsFree() && !handle.IsNew() {
			switch t := handle.(type) {
			case *mouseHandleData:
				if t.Btn == btn {
					if s != 0 {
						t.Callback(&MouseCallbackData{
							CbEvType: CALLBACK_EVENT_MOUSEBTNPUSH,
							X:        int(x),
							Y:        int(y),
						})
						t.State = byte(s)
					} else {
						t.Callback(&MouseCallbackData{
							CbEvType: CALLBACK_EVENT_MOUSEBTNRELEASE,
							X:        int(x),
							Y:        int(y),
						})
						if t.State != byte(s) {
							t.Callback(&MouseCallbackData{
								CbEvType: CALLBACK_EVENT_MOUSEBTNCLICK,
								X:        int(x),
								Y:        int(y),
							})
						}
						t.State = byte(s)
					}
				}
			}
		}
	}
	ui.mut.Unlock()
}

func (ui *UserInputStruct) MouseMotionUpdate(x int32, y int32) {
	ui.mut.Lock()
	for _, handle := range ui.handlers {
		if !handle.IsFree() && !handle.IsNew() {
			switch t := handle.(type) {
			case *mouseHandleData:
				t.Callback(&MouseCallbackData{
					CbEvType: CALLBACK_EVENT_MOUSEMOTION,
					X:        int(x),
					Y:        int(y),
				})
			}
		}
	}
	ui.mut.Unlock()
}

func (ui *UserInputStruct) WindowUpdate(evType CallbackEventType) {
	ui.mut.Lock()
	for _, handle := range ui.handlers {
		if !handle.IsFree() && !handle.IsNew() {
			switch t := handle.(type) {
			case *mouseHandleData:
				t.Callback(&WindowCallbackData{
					CbEvType: CALLBACK_EVENT_WINDOWLEAVE,
				})
			}
		}
	}
	ui.mut.Unlock()
}

func (ui *UserInputStruct) PutKeyboardCallback(key byte, callback InputCallback, systemEv bool) CallbackHandle {
	f := func(hd *keyboardHandleData) {
		hd.Key = key
		hd.Callback = callback
		hd.State = gkstate(key)
		hd.NewEv = true
		hd.SystemEv = systemEv
	}
	for h, hd := range ui.handlers {
		switch t := hd.(type) {
		case *keyboardHandleData:
			if t.Callback == nil {
				f(t)
				return h
			}
		}
	}
	cbh := CallbackHandle(len(ui.handlers))
	khd := keyboardHandleData{}
	f(&khd)
	ui.handlers[cbh] = &khd
	return cbh
}

func (ui *UserInputStruct) RemoveKeyboardCallback(bcHandle CallbackHandle) {
	ui.removeCallback(bcHandle, CALLBACK_TYPE_KEYBOARD)
}

func (ui *UserInputStruct) PutMouseBtnCallback(btn byte, callback InputCallback) CallbackHandle {
	f := func(hd *mouseHandleData) {
		hd.Btn = btn
		hd.Callback = callback
		hd.NewEv = true
	}
	for h, hd := range ui.handlers {
		switch t := hd.(type) {
		case *mouseHandleData:
			if t.Callback == nil {
				f(t)
				return h
			}
		}
	}
	cbh := CallbackHandle(len(ui.handlers))
	bhd := mouseHandleData{}
	f(&bhd)
	ui.handlers[cbh] = &bhd
	return cbh
}

func (ui *UserInputStruct) RemoveMouseBtnCallback(bcHandle CallbackHandle) {
	ui.removeCallback(bcHandle, CALLBACK_TYPE_MOUSE)
}

func (ui *UserInputStruct) PutMouseMotionCallback(callback InputCallback) CallbackHandle {
	for h, hd := range ui.handlers {
		switch t := hd.(type) {
		case *mouseHandleData:
			if t.Callback == nil {
				t.Callback = callback
				t.NewEv = true
				return h
			}
		}
	}
	cbh := CallbackHandle(len(ui.handlers))
	bhd := mouseHandleData{}
	bhd.Callback = callback
	bhd.NewEv = true
	ui.handlers[cbh] = &bhd
	return cbh
}

func (ui *UserInputStruct) RemoveMouseMotionCallback(bcHandle CallbackHandle) {
	ui.removeCallback(bcHandle, CALLBACK_TYPE_MOUSE)
}

func (ui *UserInputStruct) PutWindowCallback(callback InputCallback) CallbackHandle {
	for h, hd := range ui.handlers {
		switch t := hd.(type) {
		case *windowHandleData:
			if t.Callback == nil {
				t.Callback = callback
				t.NewEv = true
				return h
			}
		}
	}
	cbh := CallbackHandle(len(ui.handlers))
	bhd := windowHandleData{}
	bhd.Callback = callback
	bhd.NewEv = true
	ui.handlers[cbh] = &bhd
	return cbh
}

func (ui *UserInputStruct) RemoveWindowCallback(bcHandle CallbackHandle) {
	ui.removeCallback(bcHandle, CALLBACK_TYPE_WINDOW)
}

func GetKeyState(vkey byte) uint16 {
	r0, _, _ := EntryGetKeyState.Call(uintptr(vkey))
	return uint16(r0)
}

func (ui *UserInputStruct) removeCallback(handle CallbackHandle, callbackType CallbackType) {
	switch callbackType {
	case CALLBACK_TYPE_KEYBOARD:
		v, _ := ui.handlers[handle].(*keyboardHandleData)
		v.Callback = nil
	case CALLBACK_TYPE_MOUSE:
		v, _ := ui.handlers[handle].(*mouseHandleData)
		v.Callback = nil
	case CALLBACK_TYPE_WINDOW:
		v, _ := ui.handlers[handle].(*windowHandleData)
		v.Callback = nil
	}
}
