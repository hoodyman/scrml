package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/hoodyman/screenshot"
	"github.com/veandco/go-sdl2/sdl"
)

var UserGui GuiStruct

type screenIndexType int

const (
	SCREEN_INDEX_NONE screenIndexType = iota
	SCREEN_INDEX_MAIN
	SCREEN_INDEX_MARKUP
	SCREEN_INDEX_HELP
	SCREEN_INDEX_SELECTWND
	SCREEN_INDEX_NEWMARKED
)

type GuiStruct struct {
	screenIndex         screenIndexType
	screenStack         map[int]*GuiStackItem
	screenSelectWndData screenSelectWndStruct
	screenMainData      screenMainStruct
	screenMarkupData    screenMarkupStruct
	screenHelpData      screenHelpStruct
	screenNewMarked     screenNewMarkedStruct
	texUI               GuiSDLTextureMetaStruct
	texCaptured         GuiSDLTextureMetaStruct
	background0         *color.RGBA
	lockPredict         bool
}

type GuiSDLTextureMetaStruct struct {
	oldImgW    int
	oldImgH    int
	sdlTexture *sdl.Texture
}

type GuiFunction func()

type GuiStackItem struct {
	originScreenIndex   screenIndexType
	originSetFunction   GuiFunction
	targetUnsetFunction GuiFunction
}

func (gs *GuiStruct) CallScreen(targetScreenIndex screenIndexType, originSetFunction GuiFunction, originUnsetFunction GuiFunction, targetSetFunction GuiFunction, targetUnsetFunction GuiFunction) {
	s := GuiStackItem{}
	s.originScreenIndex = UserGui.screenIndex
	s.originSetFunction = originSetFunction
	s.targetUnsetFunction = targetUnsetFunction
	if originUnsetFunction != nil {
		originUnsetFunction()
	}
	targetSetFunction()
	gs.screenStack[len(gs.screenStack)] = &s
	UserGui.screenIndex = targetScreenIndex
}

func (gs *GuiStruct) ReturnScreen() {
	s := gs.screenStack[len(gs.screenStack)-1]
	delete(gs.screenStack, len(gs.screenStack)-1)
	s.targetUnsetFunction()
	if s.originSetFunction != nil {
		s.originSetFunction()
	}
	UserGui.screenIndex = s.originScreenIndex
}

func (g *GuiStruct) Init() {
	g.screenStack = make(map[int]*GuiStackItem)
	g.background0 = &color.RGBA{10, 10, 10, 220}
}

func (g *GuiStruct) RenderAndHandle(r *sdl.Renderer) {
	switch g.screenIndex {
	case SCREEN_INDEX_NONE:
		g.CallScreen(SCREEN_INDEX_MAIN, nil, nil, g.setGuiMain, g.unsetGuiMain)
	case SCREEN_INDEX_MAIN:
		g.renderGuiMain(r)
	case SCREEN_INDEX_MARKUP:
		g.renderGuiMarkup(r)
	case SCREEN_INDEX_HELP:
		g.renderGuiHelp(r)
	case SCREEN_INDEX_SELECTWND:
		g.renderGuiSelectWnd(r)
	case SCREEN_INDEX_NEWMARKED:
		g.renderGuiNewMarked(r)
	}
}

// MAIN BEGIN
type screenMainStruct struct {
	mainEnterMarkup    CallbackHandle
	mainEnterHelp      CallbackHandle
	mainEnterSelectWnd CallbackHandle
	mainMakeScreenshot CallbackHandle
	mainLearn          CallbackHandle
	mainNewMarked      CallbackHandle
	mainLearnLock      bool
	mainAction         string
}

func (g *GuiStruct) setGuiMain() {
	fEnterMarkup := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.CallScreen(SCREEN_INDEX_MARKUP, g.setGuiMain, g.unsetGuiMain, g.setGuiMarkup, g.unsetGuiMarkup)
		}
	}
	g.screenMainData.mainEnterMarkup = UserInput.PutKeyboardCallback(Config.Keybindings.Main.Markup[0], fEnterMarkup, false)
	fEnterHelp := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.CallScreen(SCREEN_INDEX_HELP, g.setGuiMain, g.unsetGuiMain, g.setGuiHelp, g.unsetGuiHelp)
		}
	}
	g.screenMainData.mainEnterHelp = UserInput.PutKeyboardCallback(Config.Keybindings.Main.Help[0], fEnterHelp, false)
	fEnterSelectWnd := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.CallScreen(SCREEN_INDEX_SELECTWND, g.setGuiMain, g.unsetGuiMain, g.setGuiSelectWnd, g.unsetGuiSelectWnd)
		}
	}
	g.screenMainData.mainEnterSelectWnd = UserInput.PutKeyboardCallback(Config.Keybindings.Main.Window[0], fEnterSelectWnd, false)
	fEnterNewMarked := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.CallScreen(SCREEN_INDEX_NEWMARKED, g.setGuiMain, g.unsetGuiMain, g.setGuiNewMarked, g.unsetGuiNewMarked)
		}
	}
	g.screenMainData.mainNewMarked = UserInput.PutKeyboardCallback(Config.Keybindings.Main.SaveNewMarked[0], fEnterNewMarked, false)
	fMakeScreenshot := func(cbData InputCallbackDataI) {
		g.screenMainData.mainAction = ": SAVING SCREENSHOT"
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_SYSTEM_KEYDOWN {
			img := ImageBuffer.Get()
			if img != nil {
				go func() {
					File.SaveCapturedImage(img)
				}()
			}
		}
		g.screenMainData.mainAction = ""
	}
	g.screenMainData.mainMakeScreenshot = UserInput.PutKeyboardCallback(Config.Keybindings.Main.Screenshot[0], fMakeScreenshot, true)
	fLearn := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			if !g.screenMainData.mainLearnLock {
				g.screenMainData.mainLearnLock = true
				f := func() {
					captureTickerSetBigInterval()
					defer func() {
						g.screenMainData.mainAction = ""
						g.screenMainData.mainLearnLock = false
						captureTickerSetNormalInterval()
						GuiTextView.Clean()
					}()
					if Ml.ServerConnected() {
						g.screenMainData.mainAction = ": LEARNING"
						Ml.Lock()
						defer Ml.Unlock()
						l := File.GetMarkedDataList()
						if err := Ml.GRPCInitMlParams(); err != nil {
							log.Println(err)
							return
						}
						t0 := time.Now()
						t1 := time.Now()
						recvAccSamples := 0
						var lo string
						for k, x := range l {
							img, _, s, err := File.LoadMarked(x)
							if err == nil {
								array := Ml.ImageToArray(img)
								if err := Ml.GRPCSendTrainingSampleData(array, byte(s)); err != nil {
									log.Println(err)
									return
								}
								recvAccSamples++
							} else {
								log.Println("LOAD LEARNING DATA ERROR:", err)
								return
							}
							s2 := fmt.Sprintf("Loading samples for train... %v%%", int(float64(k+1)*100/float64(len(l))))
							if s2 != lo {
								samplesPerSecond := float64(recvAccSamples) / time.Since(t1).Seconds()
								fmt.Println(s2 + fmt.Sprintf(" %.3f smpls/s", samplesPerSecond))
								GuiTextView.PutString(s2 + fmt.Sprintf(" %.3f smpls/s", samplesPerSecond))
								t1 = time.Now()
								recvAccSamples = 0
								lo = s2
							}
						}
						fmt.Printf("Loading OK, %v seconds\n", time.Since(t0).Seconds())
						GuiTextView.PutString(fmt.Sprintf("Loading OK, %v seconds\n", time.Since(t0).Seconds()))
						fmt.Printf("Train model... ")
						GuiTextView.PutString("Train model...")
						t0 = time.Now()
						if err := Ml.GRPCTrain(); err != nil {
							log.Println(err)
							return
						}
						s := fmt.Sprintf("Ok, %v seconds", time.Since(t0).Seconds())
						fmt.Println(s)
						GuiTextView.PutString(s)
					} else {
						s := fmt.Sprintf("No server connection")
						log.Println(s)
						GuiTextView.PutString(s)
					}
				}
				go f()
			}
		}
	}
	g.screenMainData.mainLearn = UserInput.PutKeyboardCallback(Config.Keybindings.Main.Learn[0], fLearn, false)
	GuiTextView.SetNumLines(TextDrawer.GetNumLines() - 3)
	GuiTextView.Clean()
}

func (g *GuiStruct) unsetGuiMain() {
	UserInput.RemoveKeyboardCallback(g.screenMainData.mainEnterMarkup)
	UserInput.RemoveKeyboardCallback(g.screenMainData.mainEnterHelp)
	UserInput.RemoveKeyboardCallback(g.screenMainData.mainEnterSelectWnd)
	UserInput.RemoveKeyboardCallback(g.screenMainData.mainMakeScreenshot)
	UserInput.RemoveKeyboardCallback(g.screenMainData.mainNewMarked)
	UserInput.RemoveKeyboardCallback(g.screenMainData.mainLearn)
}

func (g *GuiStruct) renderGuiMain(renderer *sdl.Renderer) {
	if ImageBuffer.Get() != nil {
		g.renderImageWithAspect(renderer, &g.texCaptured)

		g.predictCycle()
	}

	if Grid.TryLockOuter() {
		for i := range Grid.Highlighted.Selected() {
			r := Grid.TargetSdlRect(i)
			d := Grid.Highlighted.DataFromSelected(i)
			v := d.Value
			renderer.SetDrawColor(0xff, 0x00, 0x00, byte(v*0.9*255))
			renderer.FillRect(r)
		}
		Grid.UnlockOuter()
	}

	TextDrawer.PrepareDrawing()
	TextDrawer.Draw(fmt.Sprintf("PROCESSING%v", g.screenMainData.mainAction), 1, 0)
	n := 2
	for _, l := range GuiTextView.GetLines() {
		n++
		TextDrawer.Draw(l, 1, n)
	}
	img := TextDrawer.GetResultRBGA()
	if img == nil {
		return
	}
	g.renderImage(img, renderer, nil, &g.texUI)
}

func (g *GuiStruct) predictCycle() {
	if !g.lockPredict {
		g.lockPredict = true
		f := func() {
			defer func() { g.lockPredict = false }()
			if Ml.ServerConnected() && ImageBuffer.Get() != nil {
				Ml.Lock()
				if err := Ml.GRPCInitMlParams(); err == nil {
					fmt.Printf("Loading samples... ")
					t0 := time.Now()
					for i := 0; i < Grid.NumRects(); i++ {
						rect := Grid.SourceRect(i)
						sub_img := ImageBuffer.GetSub(rect)
						array := Ml.ImageToArray(sub_img)
						if err := Ml.GRPCSendPredictSampleData(array); err != nil {
							break
						}
					}
					fmt.Printf("Ok, %.3f seconds\n", time.Since(t0).Seconds())
					fmt.Printf("Prediction... ")
					t0 = time.Now()
					data, err := Ml.GRPCPredict()
					fmt.Printf("Ok, %.3f seconds\n", time.Since(t0).Seconds())
					if err != nil {
						log.Println(err)
					} else {
						Grid.LockOuter()
						Grid.Highlighted.DeselectAll()
						for i, v := range data {
							Grid.Highlighted.Select(i, &SelectedData{Value: float64(v) / 255.0})
						}
						Grid.UnlockOuter()
					}
				}
				Ml.Unlock()
			}
		}
		go f()
	}
}

// MAIN END

// MARKUP BEGIN
type screenMarkupStruct struct {
	markupPosBtn          CallbackHandle
	markupNegBtn          CallbackHandle
	markupMouseMotion     CallbackHandle
	markupExitKey         CallbackHandle
	markupEnterHelp       CallbackHandle
	markupSaveMarkupKey   CallbackHandle
	markupIgnoreMarkupKey CallbackHandle
	markupBrushModePaint  bool
	markupBrushBrushed    map[int]byte
	markupScrShotList     []string
	markupAction          string

	markupMode int
}

func (g *GuiStruct) setGuiMarkup() {
	ImageBuffer.LockOuter()
	localDeleteImage := func() {
		if len(g.screenMarkupData.markupScrShotList) != 0 {
			err := File.DeleteFile(g.screenMarkupData.markupScrShotList[0])
			if err != nil {
				log.Println("DELETE FILE ERROR:", err)
			}
			g.screenMarkupData.markupScrShotList = g.screenMarkupData.markupScrShotList[1:]
		}
	}
	localLoadImageOrDelete := func() error {
		for {
			if len(g.screenMarkupData.markupScrShotList) != 0 {
				img, err := File.LoadImage(g.screenMarkupData.markupScrShotList[0])
				if err != nil {
					log.Println("LOAD SCREENSHOT ERROR:", err)
					localDeleteImage()
				} else {
					ImageBuffer.Put(img)
					g.screenMarkupData.markupAction = g.screenMarkupData.markupScrShotList[0]
					Grid.LockOuter()
					Grid.SamplePositive.DeselectAll()
					Grid.SampleNegative.DeselectAll()
					Grid.UnlockOuter()
					return nil
				}
			} else {
				return fmt.Errorf("no files")
			}
		}
	}

	d := SelectedData{Value: 0xff / 2}

	fBtn1 := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*MouseCallbackData)
		if t.CbEvType == CALLBACK_EVENT_MOUSEBTNPUSH {
			g.screenMarkupData.markupMode = 1
			g.screenMarkupData.markupBrushBrushed = make(map[int]byte)
			n := Grid.RectAtTarget(int(t.X), int(t.Y))
			if n != -1 {
				Grid.LockOuter()
				g.screenMarkupData.markupBrushBrushed[n] = 0
				if Grid.SampleNegative.IsSelected(n) {
					Grid.SampleNegative.Deselect(n)
				}
				if Grid.SamplePositive.IsSelected(n) {
					g.screenMarkupData.markupBrushModePaint = false
					Grid.SamplePositive.Deselect(n)
					Grid.UnlockOuter()
					return
				}
				g.screenMarkupData.markupBrushModePaint = true
				Grid.SamplePositive.Select(n, &d)
				Grid.UnlockOuter()
			}
		} else if t.CbEvType == CALLBACK_EVENT_MOUSEBTNRELEASE {
			g.screenMarkupData.markupMode = 0
			for k := range g.screenMarkupData.markupBrushBrushed {
				delete(g.screenMarkupData.markupBrushBrushed, k)
			}
		}
	}
	g.screenMarkupData.markupPosBtn = UserInput.PutMouseBtnCallback(1, fBtn1)

	fBtn3 := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*MouseCallbackData)
		if t.CbEvType == CALLBACK_EVENT_MOUSEBTNPUSH {
			g.screenMarkupData.markupMode = 3
			g.screenMarkupData.markupBrushBrushed = make(map[int]byte)
			n := Grid.RectAtTarget(int(t.X), int(t.Y))
			if n != -1 {
				Grid.LockOuter()
				g.screenMarkupData.markupBrushBrushed[n] = 0
				if Grid.SamplePositive.IsSelected(n) {
					Grid.SamplePositive.Deselect(n)
				}
				if Grid.SampleNegative.IsSelected(n) {
					g.screenMarkupData.markupBrushModePaint = false
					Grid.SampleNegative.Deselect(n)
					Grid.UnlockOuter()
					return
				}
				g.screenMarkupData.markupBrushModePaint = true
				Grid.SampleNegative.Select(n, &d)
				Grid.UnlockOuter()
			}
		} else if t.CbEvType == CALLBACK_EVENT_MOUSEBTNRELEASE {
			g.screenMarkupData.markupMode = 0
			for k := range g.screenMarkupData.markupBrushBrushed {
				delete(g.screenMarkupData.markupBrushBrushed, k)
			}
		}
	}
	g.screenMarkupData.markupNegBtn = UserInput.PutMouseBtnCallback(3, fBtn3)

	fMMotion := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*MouseCallbackData)
		n := Grid.RectAtTarget(int(t.X), int(t.Y))
		if g.screenMarkupData.markupMode != 0 && n != -1 {
			if _, ok := g.screenMarkupData.markupBrushBrushed[n]; !ok {
				if g.screenMarkupData.markupBrushModePaint {
					Grid.LockOuter()
					switch g.screenMarkupData.markupMode {
					case 1:
						if !Grid.SamplePositive.IsSelected(n) {
							Grid.SamplePositive.Select(n, &d)
						}
						if Grid.SampleNegative.IsSelected(n) {
							Grid.SampleNegative.Deselect(n)
						}
					case 3:
						if !Grid.SampleNegative.IsSelected(n) {
							Grid.SampleNegative.Select(n, &d)
						}
						if Grid.SamplePositive.IsSelected(n) {
							Grid.SamplePositive.Deselect(n)
						}
					}
					Grid.UnlockOuter()
				} else {
					Grid.LockOuter()
					switch g.screenMarkupData.markupMode {
					case 1:
						if Grid.SamplePositive.IsSelected(n) {
							Grid.SamplePositive.Deselect(n)
						}
					case 3:
						if Grid.SampleNegative.IsSelected(n) {
							Grid.SampleNegative.Deselect(n)
						}
					}
					Grid.UnlockOuter()
				}
				g.screenMarkupData.markupBrushBrushed[n] = 0
			}
		}
	}
	g.screenMarkupData.markupMouseMotion = UserInput.PutMouseMotionCallback(fMMotion)

	fExitKey := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.guiMarkupExit()
		}
	}
	g.screenMarkupData.markupExitKey = UserInput.PutKeyboardCallback(Config.Keybindings.Markup.Quit[0], fExitKey, false)

	fEnterHelp := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.CallScreen(SCREEN_INDEX_HELP, g.setGuiMarkup, g.unsetGuiMarkup, g.setGuiHelp, g.unsetGuiHelp)
		}
	}
	g.screenMarkupData.markupEnterHelp = UserInput.PutKeyboardCallback(Config.Keybindings.Markup.Help[0], fEnterHelp, false)

	fSaveMarkup := func(cbData InputCallbackDataI) {
		saveHelper := func(i int, isPositive bool) {
			rect := Grid.SourceRect(i)
			sub_img := ImageBuffer.GetSub(rect)
			sub_img_resized := Image.Resize(sub_img, MarkedImageSizePixels)
			sub_img = nil
			var f_sub_name string
			if len(g.screenMarkupData.markupScrShotList) != 0 {
				f_sub_name = strings.TrimSuffix(filepath.Base(g.screenMarkupData.markupScrShotList[0]), filepath.Ext(g.screenMarkupData.markupScrShotList[0]))
			} else {
				f_sub_name = File.CreateBaseName()
			}
			if Config.Common.SaveMarkedToPersistent[0] == '1' {
				File.SaveMarked(sub_img_resized, f_sub_name, i, isPositive)
			} else {
				File.SaveNewMarked(sub_img_resized, f_sub_name, i, isPositive)
			}
		}
		defer func() {
			if len(g.screenMarkupData.markupScrShotList) != 0 {
				g.screenMarkupData.markupAction = g.screenMarkupData.markupScrShotList[0]
			}
		}()
		g.screenMarkupData.markupAction = g.screenMarkupData.markupAction + ": SAVING MARKUP DATA"
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			// if Grid.SampleNegative.SelectedAmount() != 0 {
			for _, i := range Grid.SamplePositive.Selected() {
				saveHelper(i, true)
			}
			for _, i := range Grid.SampleNegative.Selected() {
				saveHelper(i, false)
			}
			// } else {
			// 	for i := 0; i < Grid.NumRects(); i++ {
			// 		sub_img_sel := Grid.SamplePositive.IsSelected(i)
			// 		saveHelper(i, sub_img_sel)
			// 	}
			// }
			localDeleteImage()
			if localLoadImageOrDelete() != nil {
				g.guiMarkupExit()
			}
			g.screenMarkupData.markupAction = ""
		}
	}

	g.screenMarkupData.markupSaveMarkupKey = UserInput.PutKeyboardCallback(Config.Keybindings.Markup.SaveMarkup[0], fSaveMarkup, false)
	fIgnoreMarkup := func(cbData InputCallbackDataI) {
		Grid.Highlighted.DeselectAll()
		if len(g.screenMarkupData.markupScrShotList) == 0 {
			g.guiMarkupExit()
		}
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			localDeleteImage()
			if localLoadImageOrDelete() != nil {
				g.guiMarkupExit()
			}
		}
	}
	g.screenMarkupData.markupIgnoreMarkupKey = UserInput.PutKeyboardCallback(Config.Keybindings.Markup.Ignore[0], fIgnoreMarkup, false)

	g.screenMarkupData.markupScrShotList = File.GetScreenshotList()
	if len(g.screenMarkupData.markupScrShotList) != 0 {
		if localLoadImageOrDelete() != nil {
			g.guiMarkupExit()
		}
	}

}

func (g *GuiStruct) guiMarkupExit() {
	Grid.LockOuter()
	Grid.SamplePositive.DeselectAll()
	Grid.SampleNegative.DeselectAll()
	Grid.UnlockOuter()
	g.ReturnScreen()
}

func (g *GuiStruct) unsetGuiMarkup() {
	UserInput.RemoveMouseBtnCallback(g.screenMarkupData.markupPosBtn)
	UserInput.RemoveMouseBtnCallback(g.screenMarkupData.markupNegBtn)
	UserInput.RemoveMouseMotionCallback(g.screenMarkupData.markupMouseMotion)
	UserInput.RemoveKeyboardCallback(g.screenMarkupData.markupExitKey)
	UserInput.RemoveKeyboardCallback(g.screenMarkupData.markupEnterHelp)
	UserInput.RemoveKeyboardCallback(g.screenMarkupData.markupSaveMarkupKey)
	UserInput.RemoveKeyboardCallback(g.screenMarkupData.markupIgnoreMarkupKey)
	ImageBuffer.UnlockOuter()
}

func (g *GuiStruct) renderGuiMarkup(renderer *sdl.Renderer) {
	if ImageBuffer.Get() == nil {
		g.ReturnScreen()
		return
	}
	g.renderImageWithAspect(renderer, &g.texCaptured)

	g.predictCycle()

	// Grid.
	renderer.SetDrawColor(0xff, 0x00, 0x00, 0xff)
	for n := 0; n < Grid.NumRects(); n++ {
		r := Grid.TargetSdlRect(n)
		renderer.DrawRect(r)
	}
	Grid.LockOuter()
	highlightedGrid := Grid.Highlighted.Selected()
	for _, n := range highlightedGrid {
		r := Grid.TargetSdlRect(n)
		expect := Grid.Highlighted.DataFromSelected(n).Value
		b := byte(float64(0xff) * expect)
		renderer.SetDrawColor(0xff, 0x00, 0x00, b/2)
		renderer.FillRect(r)
	}
	positiveGrid := Grid.SamplePositive.Selected()
	for _, n := range positiveGrid {
		r := Grid.TargetSdlRect(n)
		expect := Grid.SamplePositive.DataFromSelected(n).Value
		b := byte(float64(0xff) * expect)
		renderer.SetDrawColor(0x00, 0xff, 0x00, b/2)
		renderer.FillRect(r)
	}
	negativeGrid := Grid.SampleNegative.Selected()
	for _, n := range negativeGrid {
		r := Grid.TargetSdlRect(n)
		expect := Grid.SampleNegative.DataFromSelected(n).Value
		b := byte(float64(0xff) * expect)
		renderer.SetDrawColor(0x00, 0x00, 0xff, b/2)
		renderer.FillRect(r)
	}
	Grid.UnlockOuter()

	// Caption.
	TextDrawer.PrepareDrawing()
	TextDrawer.Draw(fmt.Sprintf("MARKUP: %v", g.screenMarkupData.markupAction), 1, 0)
	img := TextDrawer.GetResultRBGA()
	if img == nil {
		return
	}
	g.renderImage(img, renderer, nil, &g.texUI)
}

// MARKUP END

// HELP BEGIN
type screenHelpStruct struct {
	varHelpExitKey CallbackHandle
}

func (g *GuiStruct) setGuiHelp() {
	fExit := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.ReturnScreen()
		}
	}
	g.screenHelpData.varHelpExitKey = UserInput.PutKeyboardCallback(Config.Keybindings.Help.Quit[0], fExit, false)
}

func (g *GuiStruct) unsetGuiHelp() {
	UserInput.RemoveKeyboardCallback(g.screenHelpData.varHelpExitKey)
}

func (g *GuiStruct) renderGuiHelp(renderer *sdl.Renderer) {
	g.renderImageWithAspect(renderer, &g.texCaptured)

	TextDrawer.PrepareDrawing()
	TextDrawer.SetBackgroundColor(*g.background0)
	TextDrawer.PrepareBackground()
	TextDrawer.Draw("HELP", 1, 0)
	n := 2
	for _, l := range Config.Description() {
		n++
		TextDrawer.Draw(l, 1, n)
	}
	img := TextDrawer.GetResultRBGA()
	if img == nil {
		return
	}
	g.renderImage(img, renderer, nil, &g.texUI)
}

// HELP END

// SELECT WINDOW BEGIN
type screenSelectWndStruct struct {
	selectWndExit         CallbackHandle
	selectWndMouseMove    CallbackHandle
	selectWndMouseClick   CallbackHandle
	selectWndEnterHelp    CallbackHandle
	selectWndSelectedLine int
	selectWndList         []screenshot.WindowStruct
	selectWndTitle        int
}

func (g *GuiStruct) setGuiSelectWnd() {
	g.screenSelectWndData.selectWndList = make([]screenshot.WindowStruct, 0)
	g.screenSelectWndData.selectWndSelectedLine = -1
	g.screenSelectWndData.selectWndTitle = -1

	fExit := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.ReturnScreen()
		}
	}
	g.screenSelectWndData.selectWndExit = UserInput.PutKeyboardCallback(Config.Keybindings.Window.Quit[0], fExit, false)
	fEnterHelp := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			g.CallScreen(SCREEN_INDEX_HELP, g.setGuiSelectWnd, g.unsetGuiSelectWnd, g.setGuiHelp, g.unsetGuiHelp)
		}
	}
	g.screenSelectWndData.selectWndEnterHelp = UserInput.PutKeyboardCallback(Config.Keybindings.Window.Help[0], fEnterHelp, false)
	fMouseMove := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*MouseCallbackData)
		if t.CbEvType == CALLBACK_EVENT_MOUSEMOTION {
			g.screenSelectWndData.selectWndSelectedLine = TextDrawer.InLineY(t.Y)
		}
	}
	g.screenSelectWndData.selectWndMouseMove = UserInput.PutMouseMotionCallback(fMouseMove)
	fBtn0 := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*MouseCallbackData)
		if t.CbEvType == CALLBACK_EVENT_MOUSEBTNPUSH {
			if len(g.screenSelectWndData.selectWndList) != 0 {
				if g.screenSelectWndData.selectWndTitle != -1 {
					targetWindowTitle = g.screenSelectWndData.selectWndList[g.screenSelectWndData.selectWndTitle].Title
					breakScreenshot = true
					g.ReturnScreen()
				}
			}
		}
	}
	g.screenSelectWndData.selectWndMouseClick = UserInput.PutMouseBtnCallback(1, fBtn0)
}

func (g *GuiStruct) unsetGuiSelectWnd() {
	UserInput.RemoveKeyboardCallback(g.screenSelectWndData.selectWndExit)
	UserInput.RemoveKeyboardCallback(g.screenSelectWndData.selectWndEnterHelp)
	UserInput.RemoveMouseMotionCallback(g.screenSelectWndData.selectWndMouseMove)
	UserInput.RemoveMouseBtnCallback(g.screenSelectWndData.selectWndMouseClick)
}

func (g *GuiStruct) renderGuiSelectWnd(renderer *sdl.Renderer) {
	g.renderImageWithAspect(renderer, &g.texCaptured)

	TextDrawer.PrepareDrawing()
	TextDrawer.SetBackgroundColor(color.RGBA{100, 100, 100, 100})
	TextDrawer.PrepareBackground()
	TextDrawer.Draw("SELECT WINDOW", 1, 0)
	n := 2
	g.screenSelectWndData.selectWndList = screenshot.EnumWindowList()
	if g.screenSelectWndData.selectWndSelectedLine > n && g.screenSelectWndData.selectWndSelectedLine <= len(g.screenSelectWndData.selectWndList)+n {
		TextDrawer.HighlightLine(g.screenSelectWndData.selectWndSelectedLine, 0, &color.RGBA{255, 255, 0, 255 / 5})
		g.screenSelectWndData.selectWndTitle = g.screenSelectWndData.selectWndSelectedLine - 3
	} else {
		g.screenSelectWndData.selectWndSelectedLine = -1
		g.screenSelectWndData.selectWndTitle = -1
	}

	for _, l := range g.screenSelectWndData.selectWndList {
		n++
		TextDrawer.Draw(l.Title, 1, n)
	}
	img := TextDrawer.GetResultRBGA()
	if img == nil {
		return
	}
	g.renderImage(img, renderer, nil, &g.texUI)

}

// SELECT WINDOW END

// NEW MARKED BEGIN
type screenNewMarkedStruct struct {
	ExitKey CallbackHandle
}

func (g *GuiStruct) setGuiNewMarked() {
	cancelContext, cancel := context.WithCancel(context.Background())
	outerReturn := false
	mut := sync.Mutex{}
	f := func(stop context.Context) {
		Image.MoveNewMarkedToPersistent(stop)
		mut.Lock()
		if !outerReturn {
			g.ReturnScreen()
		}
		mut.Unlock()
	}
	fExit := func(cbData InputCallbackDataI) {
		t, _ := cbData.(*KeyboardCallbackData)
		if t.CbEvType == CALLBACK_EVENT_KEYDOWN {
			mut.Lock()
			outerReturn = true
			cancel()
			g.ReturnScreen()
			mut.Unlock()
		}
	}
	g.screenNewMarked.ExitKey = UserInput.PutKeyboardCallback(Config.Keybindings.SaveNewMarked.Quit[0], fExit, false)
	GuiTextView.SetNumLines(TextDrawer.GetNumLines() - 3)
	GuiTextView.Clean()
	captureTickerSetBigInterval()
	go f(cancelContext)
}

func (g *GuiStruct) unsetGuiNewMarked() {
	UserInput.RemoveKeyboardCallback(g.screenNewMarked.ExitKey)
	captureTickerSetNormalInterval()
}

func (g *GuiStruct) renderGuiNewMarked(renderer *sdl.Renderer) {
	g.renderImageWithAspect(renderer, &g.texCaptured)

	TextDrawer.PrepareDrawing()
	TextDrawer.SetBackgroundColor(*g.background0)
	TextDrawer.PrepareBackground()
	s := fmt.Sprintf("MOVING NEW MARKED TO PERSISTENT STORAGE. PRESS %c TO QUIT", Config.Keybindings.SaveNewMarked.Quit[0])
	TextDrawer.Draw(s, 1, 0)
	n := 2
	for _, l := range GuiTextView.GetLines() {
		n++
		TextDrawer.Draw(l, 1, n)
	}
	img := TextDrawer.GetResultRBGA()
	if img == nil {
		return
	}
	g.renderImage(img, renderer, nil, &g.texUI)
}

// NEW MARKED END

func (g *GuiStruct) renderImage(img *image.RGBA, renderer *sdl.Renderer, rect *sdl.Rect, texMeta *GuiSDLTextureMetaStruct) {
	img_w := img.Bounds().Size().X
	img_h := img.Bounds().Size().Y

	var err error
	if !g.imageSizeEq(img_w, img_h, texMeta) {
		err = g.recreateSDLTexture(renderer, img_w, img_h, texMeta)
	}
	texMeta.sdlTexture.Update(&sdl.Rect{X: 0, Y: 0, W: int32(img_w), H: int32(img_h)}, unsafe.Pointer(&img.Pix[0]), img.Stride)

	if err != nil {
		log.Println("texture:", err)
	}

	if rect == nil {
		renderer.Copy(texMeta.sdlTexture, nil, &sdl.Rect{X: 0, Y: 0, W: int32(img_w), H: int32(img_h)})
	} else {
		renderer.Copy(texMeta.sdlTexture, nil, rect)
	}
}

func (g *GuiStruct) renderImageWithAspect(renderer *sdl.Renderer, texMeta *GuiSDLTextureMetaStruct) {
	img := ImageBuffer.Get()
	if img != nil {
		img_w := img.Bounds().Size().X
		img_h := img.Bounds().Size().Y

		var err error
		if !g.imageSizeEq(img_w, img_h, texMeta) {
			err = g.recreateSDLTexture(renderer, img_w, img_h, texMeta)
		}
		texMeta.sdlTexture.Update(&sdl.Rect{X: 0, Y: 0, W: int32(img_w), H: int32(img_h)}, unsafe.Pointer(&img.Pix[0]), img.Stride)

		if err != nil {
			log.Println("texture error:", err)
		} else {
			imgToTargetScale = math.Min(float64(outScreenSize.Y)/float64(img_h), float64(outScreenSize.X)/float64(img_w))
			scaledImgRect.H = int32(float64(img_h) * imgToTargetScale)
			scaledImgRect.W = int32(float64(img_w) * imgToTargetScale)
			scaledImgRect.X = int32(math.Abs(float64(scaledImgRect.W-int32(outScreenSize.X))) / 2)
			scaledImgRect.Y = int32(math.Abs(float64(scaledImgRect.H-int32(outScreenSize.Y))) / 2)

			renderer.Copy(texMeta.sdlTexture, nil, &scaledImgRect)
		}
	}
}

func (g *GuiStruct) recreateSDLTexture(renderer *sdl.Renderer, img_w int, img_h int, texMeta *GuiSDLTextureMetaStruct) error {
	var err error
	if texMeta.sdlTexture != nil {
		texMeta.sdlTexture.Destroy()
	}
	texMeta.sdlTexture, err = renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STATIC, int32(img_w), int32(img_h))
	texMeta.sdlTexture.SetBlendMode(sdl.BLENDMODE_BLEND)
	return err
}

func (g *GuiStruct) imageSizeEq(new_w int, new_h int, texMeta *GuiSDLTextureMetaStruct) bool {
	r := texMeta.oldImgW == new_w && texMeta.oldImgH == new_h
	texMeta.oldImgW = new_w
	texMeta.oldImgH = new_h
	return r
}
