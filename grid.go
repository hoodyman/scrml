package main

import (
	"image"
	"math"
	"sync"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	verticalDomains = 10
)

type SelectedData struct {
	Value float64
}

type SelectableData struct {
	g    *GridStruct
	data map[int]*SelectedData
}

type GridStruct struct {
	mut                sync.Mutex
	mutOuter           sync.Mutex
	outputScreenWidth  float64
	outputScreenHeight float64
	Highlighted        *SelectableData
	SamplePositive     *SelectableData
	SampleNegative     *SelectableData
	SignLockedOuter    bool // for debug
}

var Grid GridStruct

func (g *GridStruct) InitGrid() {
	g.mut.Lock()
	Grid.Highlighted = &SelectableData{
		g:    g,
		data: make(map[int]*SelectedData),
	}
	Grid.SamplePositive = &SelectableData{
		g:    g,
		data: make(map[int]*SelectedData),
	}
	Grid.SampleNegative = &SelectableData{
		g:    g,
		data: make(map[int]*SelectedData),
	}
	g.mut.Unlock()
}

func (g *GridStruct) LockOuter() {
	g.SignLockedOuter = true
	g.mutOuter.Lock()
}

func (g *GridStruct) TryLockOuter() bool {
	b := g.mutOuter.TryLock()
	if b {
		g.SignLockedOuter = true
	}
	return b
}

func (g *GridStruct) UnlockOuter() {
	g.SignLockedOuter = false
	g.mutOuter.Unlock()
}

func (*GridStruct) GetDomainBoundsPixels() *image.Point {
	p := image.Point{}
	p.Y = ImageBuffer.Get().Rect.Dy() / verticalDomains
	p.X = int(float64(ImageBuffer.Get().Rect.Dx()) / math.Round(float64(ImageBuffer.Get().Rect.Dx()/p.Y)))
	return &p
}

func (g *GridStruct) SetOutputScreenSize(screenWidth int, screenHeight int) {
	g.mut.Lock()
	g.outputScreenWidth = float64(screenWidth)
	g.outputScreenHeight = float64(screenHeight)
	g.mut.Unlock()
}

func (g *GridStruct) NumXYRects() image.Point {
	g.mut.Lock()
	dsize := g.GetDomainBoundsPixels()
	x := image.Point{
		X: int(ImageBuffer.Get().Rect.Dx() / dsize.X),
		Y: int(ImageBuffer.Get().Rect.Dy() / dsize.Y),
	}
	g.mut.Unlock()
	return x
}

func (g *GridStruct) NumRects() int {
	xy := g.NumXYRects()
	return xy.X * xy.Y
}

func (g *GridStruct) SourceRect(index int) *image.Rectangle {
	dsize := g.GetDomainBoundsPixels()
	g.mut.Lock()
	cols := math.Round(float64(ImageBuffer.Get().Rect.Dx()) / float64(dsize.X))
	g.mut.Unlock()

	row := math.Floor(float64(index) / cols)
	col := math.Floor(float64(index) - (row * cols))

	x := col * float64(dsize.X)
	y := row * float64(dsize.Y)
	x_max := x + float64(dsize.X)
	y_max := y + float64(dsize.Y)

	r := image.Rectangle{
		image.Point{X: int(x), Y: int(y)},
		image.Point{X: int(x_max), Y: int(y_max)},
	}
	return &r
}

func (g *GridStruct) TargetRect(index int) *image.Rectangle {
	xyRects := g.NumXYRects()

	g.mut.Lock()
	scrWs := float64(ImageBuffer.Get().Rect.Dx()) * imgToTargetScale
	scrHs := float64(ImageBuffer.Get().Rect.Dy()) * imgToTargetScale
	tdx := g.outputScreenWidth
	tdy := g.outputScreenHeight
	g.mut.Unlock()

	dmnWs := scrWs / float64(xyRects.X)
	dmnHs := scrHs / float64(xyRects.Y)

	cols := scrWs / dmnWs

	row := math.Floor(float64(index) / cols)
	col := math.Floor(float64(index) - (row * cols))

	deltaX := math.Abs(scrWs-float64(tdx)) / 2
	deltaY := math.Abs(scrHs-float64(tdy)) / 2

	x := (col * dmnWs) + deltaX
	y := (row * dmnHs) + deltaY
	x_max := x + dmnWs
	y_max := y + dmnHs

	r := image.Rectangle{
		image.Point{X: int(x), Y: int(y)},
		image.Point{X: int(x_max), Y: int(y_max)},
	}
	return &r
}

func (g *GridStruct) TargetSdlRect(index int) *sdl.Rect {
	r := g.TargetRect(index)
	r2 := sdl.Rect{
		X: int32(r.Min.X),
		Y: int32(r.Min.Y),
		W: int32(r.Dx()),
		H: int32(r.Dy()),
	}
	return &r2
}

func (g *SelectableData) Select(n int, data *SelectedData) {
	g.g.mut.Lock()
	if _, ok := g.data[n]; !ok {
		g.data[n] = data
	}
	g.g.mut.Unlock()
}

func (g *SelectableData) Deselect(n int) {
	g.g.mut.Lock()
	delete(g.data, n)
	g.g.mut.Unlock()
}

func (g *SelectableData) DeselectAll() {
	g.g.mut.Lock()
	for k := range g.data {
		delete(g.data, k)
	}
	g.g.mut.Unlock()
}

func (g *SelectableData) Selected() []int {
	g.g.mut.Lock()
	s := make([]int, 0, len(g.data))
	for k := range g.data {
		s = append(s, k)
	}
	g.g.mut.Unlock()
	return s
}

func (g *SelectableData) SelectedAmount() int {
	return len(g.data)
}

func (g *GridStruct) RectAtTarget(x int, y int) int {

	g.mut.Lock()
	scrWs := float64(ImageBuffer.Get().Rect.Dx()) * imgToTargetScale
	scrHs := float64(ImageBuffer.Get().Rect.Dy()) * imgToTargetScale
	trgWs := g.outputScreenWidth
	trgHs := g.outputScreenHeight
	g.mut.Unlock()

	xyRects := g.NumXYRects()
	dmnWs := scrWs / float64(xyRects.X)
	dmnHs := scrHs / float64(xyRects.Y)

	dmnOriginX := math.Abs(scrWs-trgWs) / 2
	dmnOriginY := math.Abs(scrHs-trgHs) / 2

	if x < int(dmnOriginX) || x >= int(dmnOriginX+scrWs) ||
		y < int(dmnOriginY) || y >= int(dmnOriginY+scrHs) {
		return -1
	}

	pX := float64(x) - dmnOriginX
	pY := float64(y) - dmnOriginY

	col := math.Floor(pX / dmnWs)
	row := math.Floor(pY / dmnHs)

	n := int((row * float64(xyRects.X)) + col)

	return n
}

func (g *SelectableData) SelectAtTarget(x int, y int, data *SelectedData) {
	n := g.g.RectAtTarget(x, y)
	if n == -1 {
		return
	}
	g.Select(n, data)
}

func (g *SelectableData) IsSelected(n int) bool {
	_, ok := g.data[n]
	return ok
}

func (g *SelectableData) IsSelectedAtTarget(x int, y int) bool {
	n := g.g.RectAtTarget(x, y)
	if n == -1 {
		return false
	}
	_, ok := g.data[n]
	return ok
}

func (g *SelectableData) DataFromSelected(n int) *SelectedData {
	g.g.mut.Lock()
	x := *g.data[n]
	g.g.mut.Unlock()
	return &x
}
