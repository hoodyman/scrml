package main

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"sync"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type TextDrawerStruct struct {
	mut            sync.Mutex
	imgW, imgH     int
	resultRGBA     *image.RGBA
	backgroundRGBA *image.RGBA
	foregroundRGBA *image.RGBA
	font           *truetype.Font
	lines          int
	fontSize       float64
	dpi            float64
	fontDrawer     *font.Drawer
}

var TextDrawer TextDrawerStruct

func (td *TextDrawerStruct) PrepareBackground() {
	draw.Draw(td.resultRGBA, td.resultRGBA.Bounds(), td.backgroundRGBA, image.Point{}, draw.Src)
}

func (td *TextDrawerStruct) PrepareDrawing() {
	td.mut.Lock()
	td.resultRGBA = image.NewRGBA(image.Rect(0, 0, td.imgW, td.imgH))
	td.updateFontDrawer()
	td.mut.Unlock()
}

func (td *TextDrawerStruct) Draw(text string, posX int, posY int) {
	td.mut.Lock()

	x := int(float64(posX) * td.SymbolSize())
	y := int(td.SymbolSize()) * (posY + 1)
	td.fontDrawer.Dot = fixed.Point26_6{
		X: fixed.I(x),
		Y: fixed.I(y),
	}

	td.fontDrawer.DrawString(text)

	td.mut.Unlock()
}

func (td *TextDrawerStruct) HighlightLine(line int, hOverPix int, color *color.RGBA) {
	rect := TextDrawer.getLineBounds(line)
	rect.Min.Y = rect.Min.Y - hOverPix
	rect.Max.Y = rect.Max.Y + (hOverPix * 2)

	for i := rect.Min.Y; i <= rect.Max.Y; i++ {
		for j := rect.Min.X; j <= rect.Max.X; j++ {
			td.resultRGBA.Set(j, i, *color)
		}
	}
}

func (td *TextDrawerStruct) InLineY(Y int) int {
	line := Y / int(td.SymbolSize())
	return line
}

func (td *TextDrawerStruct) getLineBounds(line int) *image.Rectangle {
	x := 0
	y := int(td.SymbolSize())*(line+1) - int(td.SymbolSize())
	w := td.imgW
	h := int(td.SymbolSize()) + y
	return &image.Rectangle{Min: image.Point{x, y}, Max: image.Point{w, h}}
}

func (td *TextDrawerStruct) SymbolSize() float64 {
	return td.fontSize * td.dpi / 72
}

func (td *TextDrawerStruct) SetNumLines(n int) {
	td.mut.Lock()
	td.setNumLinesNoLock(n)
	td.mut.Unlock()
}

func (td *TextDrawerStruct) GetNumLines() int {
	return td.lines
}

func (td *TextDrawerStruct) GetResultRBGA() *image.RGBA {
	return td.resultRGBA
}

func (td *TextDrawerStruct) SetBackgroundColor(c color.RGBA) {
	td.mut.Lock()
	td.setBackgroundColorNoLock(c)
	td.mut.Unlock()
}

func (td *TextDrawerStruct) SetTextColor(c color.RGBA) {
	td.mut.Lock()
	td.setTextColorNoLock(c)
	td.mut.Unlock()
}

func (td *TextDrawerStruct) Init(fontPath string) error {
	td.mut.Lock()
	defer td.mut.Unlock()

	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		return err
	}

	td.font, err = truetype.Parse(fontBytes)
	if err != nil {
		return err
	}

	return nil
}

func (td *TextDrawerStruct) Update(screenW int, screenH int) {
	td.mut.Lock()
	defer td.mut.Unlock()

	td.imgW, td.imgH = screenW, screenH
	td.dpi = 72
	td.setNumLinesNoLock(40)

	td.resultRGBA = image.NewRGBA(image.Rect(0, 0, td.imgW, td.imgH))
	td.setBackgroundColorNoLock(color.RGBA{0, 0, 0, 0})
	td.setTextColorNoLock(color.RGBA{255, 0, 0, 255})

	td.updateFontDrawer()
}

func (td *TextDrawerStruct) setBackgroundColorNoLock(c color.RGBA) {
	if (td.backgroundRGBA == nil) || (td.backgroundRGBA.At(0, 0) != c) {
		td.backgroundRGBA = image.NewRGBA(image.Rect(0, 0, td.imgW, td.imgH))
		for i := 0; i < td.imgW; i++ {
			for j := 0; j < td.imgH; j++ {
				td.backgroundRGBA.Set(i, j, c)
			}
		}
	}
}

func (td *TextDrawerStruct) setTextColorNoLock(c color.RGBA) {
	if (td.foregroundRGBA == nil) || (td.foregroundRGBA.At(0, 0) != c) {
		td.foregroundRGBA = image.NewRGBA(image.Rect(0, 0, td.imgW, td.imgH))
		for i := 0; i < td.imgW; i++ {
			for j := 0; j < td.imgH; j++ {
				td.foregroundRGBA.Set(i, j, c)
			}
		}
	}
}

func (td *TextDrawerStruct) setNumLinesNoLock(n int) {
	td.lines = n
	td.fontSize = float64(td.imgH) / ((td.dpi / 72) * float64(n))
	td.updateFontDrawer()
}

func (td *TextDrawerStruct) updateFontDrawer() {
	td.fontDrawer = &font.Drawer{
		Dst: td.resultRGBA,
		Src: td.foregroundRGBA,
		Face: truetype.NewFace(td.font, &truetype.Options{
			Size:    td.fontSize,
			DPI:     td.dpi,
			Hinting: font.HintingNone,
		}),
	}
}
