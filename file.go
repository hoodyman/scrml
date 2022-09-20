package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const fileScreenshotDataFolder = "scrshotdata"
const fileMarkedDataFolder = "markeddata"
const fileNewMarkedDataFolder = "newmarkeddata"
const fileScreenshotDataSuffix = "png"

type FileStruct struct{}

var File FileStruct

func (m *FileStruct) CreateBaseName() string {
	base_name := fmt.Sprintf("%d%02d%02d%02d%02d%02d%09d",
		time.Now().UTC().Year(),
		time.Now().UTC().Month(),
		time.Now().UTC().Day(),
		time.Now().UTC().Hour(),
		time.Now().UTC().Minute(),
		time.Now().UTC().Second(),
		time.Now().UTC().Nanosecond())
	return base_name
}

func (m *FileStruct) SaveCapturedImage(img *image.RGBA) error {
	s_name := fmt.Sprintf("%v.%v", m.CreateBaseName(), fileScreenshotDataSuffix)
	err := os.MkdirAll(fileScreenshotDataFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("SaveCapturedImage error: %v", err)
	}
	err = m.SaveImage(img, path.Join(fileScreenshotDataFolder, s_name))
	if err != nil {
		return fmt.Errorf("SaveCapturedImage error: %v", err)
	}
	return nil
}

func (*FileStruct) LoadImage(filePath string) (*image.RGBA, error) {
	imgCached, ok := FileImageCache.Get(filePath)
	if ok {
		return imgCached, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("LoadImage error: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("LoadImage error: %v", err)
	}
	v, _ := img.(*image.RGBA)
	FileImageCache.Put(filePath, v)
	return v, nil
}

func (*FileStruct) SaveImage(img *image.RGBA, filePath string) error {
	FileImageCache.Put(filePath, img)
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("SaveImage error: %v", err)
	}
	defer f.Close()
	err = png.Encode(f, img)
	if err != nil {
		return fmt.Errorf("SaveImage error: %v", err)
	}
	return nil
}

func (*FileStruct) DeleteFile(path string) error {
	FileImageCache.Delete(path)
	return os.Remove(path)
}

func (*FileStruct) GetScreenshotList() []string {
	s := make([]string, 0)
	filepath.Walk(fileScreenshotDataFolder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				s = append(s, path)
			}
			return nil
		})
	return s
}

func (*FileStruct) GetMarkedDataList() []string {
	s := make([]string, 0)
	filepath.Walk(fileMarkedDataFolder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				s = append(s, path)
			}
			return nil
		})
	return s
}

func (m *FileStruct) GetMarkedDataMap() map[string]byte {
	l := m.GetMarkedDataList()
	x := make(map[string]byte)
	for _, v := range l {
		x[v] = 0
	}
	return x
}

func (*FileStruct) GetNewMarkedDataList() []string {
	s := make([]string, 0)
	filepath.Walk(fileNewMarkedDataFolder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				s = append(s, path)
			}
			return nil
		})
	return s
}

func (m *FileStruct) GetNewMarkedDataMap() map[string]byte {
	l := m.GetNewMarkedDataList()
	x := make(map[string]byte)
	for _, v := range l {
		x[v] = 0
	}
	return x
}

// image, index, selected, error
func (m *FileStruct) LoadMarked(fPath string) (*image.RGBA, int, int, error) {
	img, err := m.LoadImage(fPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("LoadMarking error: %v", err)
	}
	x := strings.TrimSuffix(filepath.Base(fPath), filepath.Ext(fPath))
	_selected, err := strconv.Atoi(filepath.Ext(x)[1:])
	if err != nil {
		return nil, 0, 0, fmt.Errorf("MARKED FILE %v SELECTED SIGN ERROR: %v", filepath.Base(fPath), err)
	}
	x = strings.TrimSuffix(x, filepath.Ext(x))
	_index, err := strconv.Atoi(filepath.Ext(x)[1:])
	if err != nil {
		return nil, 0, 0, fmt.Errorf("MARKED FILE %v INDEX ERROR: %v", filepath.Base(fPath), err)
	}
	return img, _index, _selected, nil
}

func (m *FileStruct) SaveImageToPersistent(img *image.RGBA, name string) error {
	err := os.MkdirAll(fileMarkedDataFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("SaveMarked error: %v", err)
	}
	err = m.SaveImage(img, path.Join(fileMarkedDataFolder, name))
	if err != nil {
		return fmt.Errorf("SaveMarked error: %v", err)
	}
	return nil
}

func (m *FileStruct) SaveMarked(img *image.RGBA, baseName string, index int, selected bool) error {
	var sel int
	if selected {
		sel = 1
	}
	s_name := fmt.Sprintf("%s.%d.%d.%s",
		baseName,
		index,
		sel,
		fileScreenshotDataSuffix)
	err := os.MkdirAll(fileMarkedDataFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("SaveMarked error: %v", err)
	}
	err = m.SaveImage(img, path.Join(fileMarkedDataFolder, s_name))
	if err != nil {
		return fmt.Errorf("SaveMarked error: %v", err)
	}
	return nil
}

func (m *FileStruct) SaveNewMarked(img *image.RGBA, baseName string, index int, selected bool) error {
	var sel int
	if selected {
		sel = 1
	}
	s_name := fmt.Sprintf("%s.%d.%d.%s",
		baseName,
		index,
		sel,
		fileScreenshotDataSuffix)
	err := os.MkdirAll(fileNewMarkedDataFolder, os.ModeDir)
	if err != nil {
		return fmt.Errorf("SaveNewMarked error: %v", err)
	}
	err = m.SaveImage(img, path.Join(fileNewMarkedDataFolder, s_name))
	if err != nil {
		return fmt.Errorf("SaveNewMarked error: %v", err)
	}
	return nil
}
