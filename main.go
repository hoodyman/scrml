package main

import (
	"fmt"
	"image"
	"log"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/hoodyman/screenshot"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	drawFPS = 30
)

var (
	targetWindowTitle    string
	breakScreenshot      bool
	outScreenSize        image.Point
	stopChan             = make(chan int, 3)
	ImageBuffer          = ImageBufferStruct{}
	imgToTargetScale     float64
	scaledImgRect        = sdl.Rect{}
	captureTicker        *time.Ticker
	captureTickerSetLock sync.Mutex
)

type ImageBufferStruct struct {
	mut       sync.Mutex
	mut_outer sync.Mutex
	image     *image.RGBA
}

func (m *ImageBufferStruct) LockOuter() {
	m.mut_outer.Lock()
}

func (m *ImageBufferStruct) UnlockOuter() {
	m.mut_outer.Unlock()
}

func (ib *ImageBufferStruct) Put(img *image.RGBA) {
	ib.mut.Lock()
	ib.image = img
	ib.mut.Unlock()
}

func (ib *ImageBufferStruct) Get() *image.RGBA {
	ib.mut.Lock()
	if ib.image != nil {
		x := ib.image
		ib.mut.Unlock()
		return x
	}
	ib.mut.Unlock()
	return nil
}

func (ib *ImageBufferStruct) GetSub(rect *image.Rectangle) *image.RGBA {
	ib.mut.Lock()
	if ib.image != nil {
		x := ib.image
		z := x.SubImage(*rect)
		y, ok := z.(*image.RGBA)
		if !ok {
			ib.mut.Unlock()
			return nil
		}
		ib.mut.Unlock()
		return y
	}
	ib.mut.Unlock()
	return nil
}

func GoRectToSDLRect(rect *image.Rectangle) *sdl.Rect {
	return &sdl.Rect{X: int32(rect.Min.X), Y: int32(rect.Min.Y), W: int32(rect.Max.X), H: int32(rect.Max.Y)}
}

func captureTickerSetNormalInterval() {
	captureTickerSetLock.Lock()
	if captureTicker != nil {
		captureTicker.Reset(time.Second / drawFPS)
	} else {
		captureTicker = time.NewTicker(time.Second / drawFPS)
	}
	captureTickerSetLock.Unlock()
}

func captureTickerSetBigInterval() {
	const fps = 3
	captureTickerSetLock.Lock()
	if captureTicker != nil {
		captureTicker.Reset(time.Second / fps)
	} else {
		captureTicker = time.NewTicker(time.Second / fps)
	}
	captureTickerSetLock.Unlock()
}

func scrcapturer(stop chan int) {
	captureTickerSetNormalInterval()
	var scrshotState *screenshot.ScreenshotState
	for {
		select {
		case <-stop:
			captureTicker.Stop()
			if scrshotState != nil {
				scrshotState.Destroy()
			}
			return
		case <-captureTicker.C:
			ImageBuffer.LockOuter()
			if breakScreenshot {
				if scrshotState != nil {
					scrshotState.Destroy()
					scrshotState = nil
				}
				breakScreenshot = false
			} else if scrshotState != nil {
				img, err := scrshotState.MakeScreenshot()
				if err == nil {
					ImageBuffer.Put(img)
				} else {
					scrshotState.Destroy()
					scrshotState = nil
				}
			} else {
				if len(targetWindowTitle) != 0 {
					scrshotState, _ = screenshot.CreateStateWindow(targetWindowTitle)
					if scrshotState != nil {
						captureTickerSetNormalInterval()
					} else {
						captureTickerSetBigInterval()
					}
				}
			}
			ImageBuffer.UnlockOuter()
		}
	}
}

func scrdrawer(stop chan int, wnd *sdl.Window) {
	ticker := time.NewTicker(time.Second / drawFPS)
	renderer, err := sdl.CreateRenderer(wnd, -1, 0)
	if err != nil {
		log.Println("get renderer:", err)
		return
	}
	defer renderer.Destroy()
	renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	for {
		select {
		case <-stop:
			ticker.Stop()
			return
		case <-ticker.C:
			renderer.SetDrawColor(0x00, 0x00, 0x00, 0xff)
			renderer.Clear()

			UserGui.RenderAndHandle(renderer)

			renderer.Present()
		}
	}
}

func main() {
	log.SetFlags(log.Lshortfile)
	var err error

	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	FileImageCache.SetSizeLimitBytes(1024 * 1024 * 1024)

	// err = Image.CleanupMarkedData()
	// d, err := Image.DiffTwoMarkedImages("20220911110017339846000.43.0.png", "20220911110017339846000.47.0.png")
	// if err != nil {
	// 	fmt.Println(err)
	// } else {
	// 	fmt.Println("OK")
	// 	// fmt.Println(d)
	// }
	// return

	err = Config.Load()
	if err != nil {
		log.Println("Load config error:", err)
		return
	}

	// if err := Ml.StartMlServer(); err != nil {
	// 	log.Println("Start ML server error:", err)
	// 	return
	// }
	// defer Ml.StopMlServer()

	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			select {
			case <-ticker.C:
				if !Ml.ServerConnected() {
					fmt.Println("Connect to server")
					Ml.ConnectServer()
				}
			}
		}
	}()
	// defer Ml.DisconnectServer()

	Grid.InitGrid()
	UserGui.Init()
	err = TextDrawer.Init("arial.ttf")
	if err != nil {
		log.Println(err)
		return
	}

	sdl.Init(sdl.INIT_EVERYTHING)
	defer sdl.Quit()

	sdlWindow, err := sdl.CreateWindow("window", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED, 1280, 768, sdl.WINDOW_RESIZABLE)
	if err != nil {
		log.Println(err)
		return
	}
	defer sdlWindow.Destroy()

	for {
		ev := sdl.WaitEventTimeout(int(time.Millisecond))
		if ev != nil {
			switch t := ev.(type) {
			case *sdl.WindowEvent:
				switch t.Event {
				case sdl.WINDOWEVENT_SHOWN:
					go scrcapturer(stopChan)
					go scrdrawer(stopChan, sdlWindow)
					UserInput.StartUpdate()
					w, h := sdlWindow.GetSize()
					outScreenSize.X = int(w)
					outScreenSize.Y = int(h)
					TextDrawer.Update(int(w), int(h))
					Grid.SetOutputScreenSize(int(w), int(h))
				case sdl.WINDOWEVENT_CLOSE:
					stopChan <- 0 // shoter
					stopChan <- 0 // drawer
					UserInput.StopUpdate()
					return
				case sdl.WINDOWEVENT_RESIZED:
					w, h := sdlWindow.GetSize()
					outScreenSize.X = int(w)
					outScreenSize.Y = int(h)
					TextDrawer.Update(int(w), int(h))
					Grid.SetOutputScreenSize(int(w), int(h))
				}
			case *sdl.MouseButtonEvent:
				x, y, s := sdl.GetMouseState()
				UserInput.MouseBtnUpdate(t.Button, s, x, y)
			case *sdl.MouseMotionEvent:
				UserInput.MouseMotionUpdate(t.X, t.Y)
			case *sdl.KeyboardEvent:
				switch t.Type {
				case sdl.KEYDOWN:
					if t.Repeat == 0 {
						UserInput.KeyboardUpdate(sdl.GetScancodeName(t.Keysym.Scancode)[0], 1)
					}
				case sdl.KEYUP:
					UserInput.KeyboardUpdate(sdl.GetScancodeName(t.Keysym.Scancode)[0], 0)
				}
			}
		}
	}

}
