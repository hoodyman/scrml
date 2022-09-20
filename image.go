package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"math"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"golang.org/x/image/draw"
)

type ImageStruct struct{}

var Image ImageStruct

func (m *ImageStruct) Resize(img *image.RGBA, size int) *image.RGBA {
	ri := image.NewRGBA(image.Rect(0, 0, MarkedImageSizePixels, MarkedImageSizePixels))
	draw.NearestNeighbor.Scale(ri, ri.Rect, img, img.Rect, draw.Over, nil)
	return ri
}

func (m *ImageStruct) CleanupNewMarkedData(stop context.Context) error {
	fileList := File.GetNewMarkedDataMap()
	mut := sync.Mutex{}
	baseSizeFL := len(fileList)
	if len(fileList) < 2 {
		return nil
	}
	numWorkers := runtime.NumCPU()

	wg_stop := sync.WaitGroup{}
	wg_stop.Add(numWorkers)

	wg_done := sync.WaitGroup{}

	data_ch := make(chan string, numWorkers)
	stop_ch := make(chan int, numWorkers)

	w := func(stop chan int, data chan string) {
		for {
			select {
			case <-stop:
				wg_stop.Done()
				return
			case d := <-data:
				img0, err := File.LoadImage(d)
				if err != nil {
					s := fmt.Sprintf("%v", err)
					fmt.Println(s)
					GuiTextView.PutString(s)
				} else {
					mut.Lock()
					fileListClone := make([]string, 0, len(fileList))
					for k := range fileList {
						fileListClone = append(fileListClone, k)
					}
					mut.Unlock()
					for _, k := range fileListClone {
						select {
						case <-stop:
							return
						default:
							mut.Lock()
							b := fileList[k]
							mut.Unlock()
							if b == 0 {
								img1, err := File.LoadImage(k)
								if err != nil {
									s := fmt.Sprintf("%v", err)
									fmt.Println(s)
									GuiTextView.PutString(s)
								} else {
									diff := m.DiffRGBARMSSameSize(img0, img1)
									if diff < 10 {
										mut.Lock()
										fileList[k] = 1
										mut.Unlock()
									}
								}
							}
						}
					}
				}
				wg_done.Done()
			}
		}
	}

	for i := 0; i < numWorkers; i++ {
		go w(stop_ch, data_ch)
	}

	defer func() {
		for i := 0; i < numWorkers; i++ {
			stop_ch <- 0
		}
		wg_stop.Wait()
		s := "Cleaning done"
		fmt.Println(s)
		GuiTextView.PutString(s)
	}()

	filesDone := 0
	var lo float64

	for {
		select {
		case <-stop.Done():
			return nil
		default:
			if len(fileList) <= numWorkers {
				return m.CleanupNewMarkedDataOneByOne(stop, &fileList, false)
			} else {
				x := make(map[string]byte, numWorkers)
				for len(x) < numWorkers {
					for i := len(x); i < numWorkers; i++ {
						e, ok := Tools.GetMapStringByteFirstElement(&fileList)
						delete(fileList, e)
						x[e] = 0
						if !ok {
							return m.CleanupNewMarkedDataOneByOne(context.Background(), &x, false)
						}
					}
					m.CleanupNewMarkedDataOneByOne(context.Background(), &x, false)
				}
				s := "Scanning..."
				fmt.Println(s)
				GuiTextView.PutString(s)
				wg_done.Add(numWorkers)
				for k := range x {
					data_ch <- k
				}
				for k := range x {
					delete(x, k)
					filesDone++
				}
				wg_done.Wait()
				s = "Cleaning..."
				fmt.Println(s)
				GuiTextView.PutString(s)
				for k, v := range fileList {
					if v == 1 {
						File.DeleteFile(k)
						delete(fileList, k)
						s := fmt.Sprintf("Delete %v", filepath.Base(k))
						fmt.Println(s)
						GuiTextView.PutString(s)
					}
				}
			}
			xx := 100.0 - float64(len(fileList))*100/float64(baseSizeFL)
			if xx != lo {
				s := fmt.Sprintf("CleanupNewMarkedData... %.3f%%", xx)
				fmt.Println(s)
				GuiTextView.PutString(s)
				lo = xx
			}
		}
	}
}

func (m *ImageStruct) CleanupNewMarkedDataOneByOne(stop context.Context, l *map[string]byte, showProgress bool) error {
	if len(*l) < 2 {
		return nil
	}
	l2 := make(map[string]byte)
	for k, v := range *l {
		l2[k] = v
	}
	baseLength := len(l2)
	var lo float64
	for {
		e, _ := Tools.GetMapStringByteFirstElement(&l2)
		delete(l2, e)
		n := len(l2)

		if showProgress {
			d := baseLength - n
			x := float64(d+1) * 100 / float64(baseLength)
			if x != lo {
				s := fmt.Sprintf("CleanupNewMarkedDataOneByOne... %.3f%%", x)
				fmt.Println(s)
				GuiTextView.PutString(s)
				lo = x
			}
		}

		if n < 2 {
			return nil
		}
		img0, err := File.LoadImage(e)
		if err == nil {
			for k := range l2 {
				img1, err := File.LoadImage(k)
				if err == nil {
					if m.IsSameSize(img0, img1) {
						var diffRMS float64
						select {
						case <-stop.Done():
							return nil
						default:
							diffRMS = m.DiffRGBARMSSameSize(img0, img1)
						}
						if diffRMS < 10 {
							File.DeleteFile(k)
							delete(l2, k)
							delete(*l, k)
							s := fmt.Sprintf("Delete %v :: %v", filepath.Base(k), diffRMS)
							fmt.Println(s)
							GuiTextView.PutString(s)
						}
					} else {
						s := fmt.Sprintf("CleanupNewMarkedDataOneByOne error: image %v is not same size as image %v", filepath.Base(e), k)
						fmt.Println(s)
						GuiTextView.PutString(s)
						delete(l2, k)
					}
				} else {
					s := fmt.Sprintf("CleanupNewMarkedDataOneByOne img1 error: %v", err)
					fmt.Println(s)
					GuiTextView.PutString(s)
					delete(l2, k)
				}
			}
		} else {
			s := fmt.Sprintf("CleanupNewMarkedDataOneByOne img0 error: %v", err)
			fmt.Println(s)
			GuiTextView.PutString(s)
			delete(l2, e)
		}
	}
}

func (m *ImageStruct) IsSimilarImageInPersistentMarked(stop context.Context, origin_img *image.RGBA) bool {
	l := File.GetMarkedDataList()
	if len(l) == 0 {
		return false
	}
	var lo float64
	iA := 0
	iB := len(l)
	t0 := time.Now()
	fPrint := func(frc bool) {
		if time.Since(t0).Seconds() >= 1 || frc {
			t0 = time.Now()
			x := float64(iA+1) * 100 / float64(iB)
			if x != lo {
				s := fmt.Sprintf("Find similar image in persistent storage... %.3f%%", x)
				GuiTextView.PutString(s)
				fmt.Println(s)
				lo = x
			}
		}
	}
	defer fPrint(true)

	numWorkers := runtime.NumCPU()
	data_ch := make(chan *image.RGBA)
	stop_ch := make(chan int, numWorkers)
	found_ch := make(chan bool)

	fDiff := func(stop chan int, data chan *image.RGBA, found chan bool) {
		for {
			select {
			case <-stop:
				return
			case d := <-data:
				diff := m.DiffRGBARMSSameSize(origin_img, d)
				if diff < 10 {
					found <- true
				} else {
					found <- false
				}
			}
		}
	}

	for i := 0; i < numWorkers; i++ {
		go fDiff(stop_ch, data_ch, found_ch)
	}

	n := 0

	defer func() {
		for i := 0; i < numWorkers; i++ {
			stop_ch <- 0
		}
		for ; n != 0; n-- {
			<-found_ch
		}
	}()

	for i, next_image_name := range l {
		select {
		case <-stop.Done():
			return false
		default:
			iA = i
			target_image, err := File.LoadImage(next_image_name)
			if err == nil {
				if m.IsSameSize(origin_img, target_image) {
					recv := false
					for !recv {
						select {
						case b := <-found_ch:
							n--
							if b {
								return true
							}
						case data_ch <- target_image:
							n++
							recv = true
						}
					}
				} else {
					s := fmt.Sprintf("IsImageInPersistentMarked error: image %v is not same size as origin image", next_image_name)
					GuiTextView.PutString(s)
					fmt.Println(s)
				}
			} else {
				s := fmt.Sprintf("IsImageInPersistentMarked image %v error: %v", next_image_name, err)
				GuiTextView.PutString(s)
				fmt.Println(s)
			}
			fPrint(false)
		}
	}

	for ; n != 0; n-- {
		b := <-found_ch
		if b {
			n--
			return true
		}
	}

	return false
}

func (m *ImageStruct) MoveNewMarkedToPersistent(stop context.Context) error {
	m.CleanupNewMarkedData(stop)
	l := File.GetNewMarkedDataMap()
	baseSize := len(l)
	for {
		select {
		case <-stop.Done():
			return nil
		default:
		}
		if len(l) == 0 {
			break
		}
		e, _ := Tools.GetMapStringByteFirstElement(&l)
		delete(l, e)
		img, err := File.LoadImage(e)
		if err == nil {
			s := fmt.Sprintf("Find similar to %v :: %.3f%%", e, 100.0-float64(len(l)+1)*100/float64(baseSize))
			GuiTextView.PutString(s)
			fmt.Println(s)
			if m.IsSimilarImageInPersistentMarked(context.Background(), img) {
				s := fmt.Sprintf("%v has similar image in persistent storage, delete it", e)
				GuiTextView.PutString(s)
				fmt.Println(s)
				File.DeleteFile(e)
			}
		} else {
			GuiTextView.PutString(fmt.Sprintf("FAIL: open %v, %v", e, err))
			log.Println(fmt.Errorf("MoveNewMarkedToPersistent error: %v", err))
		}
	}
	l2 := File.GetNewMarkedDataList()
	for _, f := range l2 {
		select {
		case <-stop.Done():
			return nil
		default:
		}
		img, err := File.LoadImage(f)
		if err == nil {
			name := filepath.Base(f)
			err := File.SaveImageToPersistent(img, name)
			GuiTextView.PutString(fmt.Sprintf("Move %v", name))
			if err == nil {
				File.DeleteFile(f)
			} else {
				GuiTextView.PutString(fmt.Sprintf("FAIL: move %v, %v", name, err))
				log.Println(fmt.Errorf("MoveNewMarkedToPersistent error: %v", err))
			}
		} else {
			GuiTextView.PutString(fmt.Sprintf("FAIL: open %v, %v", f, err))
			log.Println(fmt.Errorf("MoveNewMarkedToPersistent error: %v", err))
		}
	}
	return nil
}

func (m *ImageStruct) DiffTwoMarkedImages(filename0 string, filename1 string) (float64, error) {
	p0 := path.Join(fileMarkedDataFolder, filename0)
	p1 := path.Join(fileMarkedDataFolder, filename1)
	img0, err := File.LoadImage(p0)
	if err != nil {
		return 0, fmt.Errorf("DiffTwoMarkedImages error: %v", err)
	}
	img1, err := File.LoadImage(p1)
	if err != nil {
		return 0, fmt.Errorf("DiffTwoMarkedImages error: %v", err)
	}
	if !m.IsSameSize(img0, img1) {
		return 0, fmt.Errorf("DiffTwoMarkedImages error: image %v is no same size as %v", p0, p1)
	}
	return m.DiffRGBARMSSameSizeMT(img0, img1), nil
}

func (m *ImageStruct) IsSameSize(img0 *image.RGBA, img1 *image.RGBA) bool {
	return img0.Rect == img1.Rect
}

func (m *ImageStruct) DiffRGBARMSSameSize(img0 *image.RGBA, img1 *image.RGBA) float64 {
	var acc int64 = 0
	var n int64 = 0

	for i := 0; i < len(img0.Pix); i += 4 {
		pI0r := int64(img0.Pix[i+0])
		pI0g := int64(img0.Pix[i+1])
		pI0b := int64(img0.Pix[i+2])

		pI1r := int64(img1.Pix[i+0])
		pI1g := int64(img1.Pix[i+1])
		pI1b := int64(img1.Pix[i+2])

		pDr := pI0r - pI1r
		pDg := pI0g - pI1g
		pDb := pI0b - pI1b

		p2r := pDr * pDr
		p2g := pDg * pDg
		p2b := pDg * pDb

		acc += p2r + p2g + p2b
		n += 3
	}

	rms := math.Sqrt(float64(acc) / float64(n))
	return rms
}

func (m *ImageStruct) DiffRGBARMSSameSizeMT(img0 *image.RGBA, img1 *image.RGBA) float64 {
	var acc int64 = 0
	var n int64 = 0
	mut := sync.Mutex{}
	wg := sync.WaitGroup{}

	worker := func(ych chan int, stop chan int) {
		var lacc int64
		var ln int64
		for {
			select {
			case <-stop:
				wg.Done()
				return
			case y := <-ych:
				for x := 0; x < img0.Rect.Dx(); x++ {
					pI0po := img0.PixOffset(x, y)
					pI1po := img1.PixOffset(x, y)

					pI0r := int64(img0.Pix[pI0po+0])
					pI0g := int64(img0.Pix[pI0po+1])
					pI0b := int64(img0.Pix[pI0po+2])

					pI1r := int64(img1.Pix[pI1po+0])
					pI1g := int64(img1.Pix[pI1po+1])
					pI1b := int64(img1.Pix[pI1po+2])

					pDr := pI0r - pI1r
					pDg := pI0g - pI1g
					pDb := pI0b - pI1b

					p2r := pDr * pDr
					p2g := pDg * pDg
					p2b := pDg * pDb

					lacc += p2r + p2g + p2b
					ln += 3
				}
				mut.Lock()
				acc += lacc
				n += ln
				mut.Unlock()
				lacc = 0
				ln = 0
			}
		}
	}

	y_ch := make(chan int)
	stop_ch := make(chan int)

	workers := runtime.NumCPU() * 2

	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go worker(y_ch, stop_ch)
	}

	for y := 0; y < img0.Rect.Dy(); y++ {
		y_ch <- y
	}

	for i := 0; i < workers; i++ {
		stop_ch <- 0
	}

	wg.Wait()

	rms := math.Sqrt(float64(acc) / float64(n))
	return rms
}
