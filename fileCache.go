package main

import (
	"image"
	"sync"
)

type FileImageCacheStruct struct {
	sizeLimitBytes   int
	storageSizeBytes int
	storage          map[string]FileImageCacheStorageItemStruct
	mut              sync.Mutex
	badBalance       bool
}

type FileImageCacheStorageItemStruct struct {
	loadCounter int
	image       *image.RGBA
}

var FileImageCache FileImageCacheStruct

func (m *FileImageCacheStruct) IsBalanceBad() bool {
	return m.badBalance
}

func (m *FileImageCacheStruct) SetSizeLimitBytes(limit int) {
	m.mut.Lock()
	m.sizeLimitBytes = limit
	m.mut.Unlock()
}

func (m *FileImageCacheStruct) StorageSize() int {
	return m.storageSizeBytes
}

func (m *FileImageCacheStruct) Get(path string) (*image.RGBA, bool) {
	m.mut.Lock()
	defer m.mut.Unlock()
	v, ok := m.storage[path]
	if !ok {
		return nil, false
	}
	img := image.NewRGBA(v.image.Rect)
	copy(img.Pix, v.image.Pix)
	v.loadCounter++
	if v.loadCounter < 0 {
		for _, v := range m.storage {
			if v.loadCounter != 0 {
				v.loadCounter--
			}
		}
	}
	return img, true
}

func (m *FileImageCacheStruct) Put(path string, img *image.RGBA) {
	m.mut.Lock()
	defer m.mut.Unlock()

	if m.storage == nil {
		m.storage = make(map[string]FileImageCacheStorageItemStruct)
	}

	if m.sizeLimitBytes < len(img.Pix) {
		return
	}

	v, ok := m.storage[path]
	if ok {
		for {
			balance := m.sizeLimitBytes - m.storageSizeBytes + len(v.image.Pix) - len(img.Pix)
			if balance >= 0 {
				copy(v.image.Pix, img.Pix)
				m.storageSizeBytes = balance
				m.badBalance = false
				return
			}
			m.badBalance = true
			if !m.removeUnpopularExclude(path) {
				delete(m.storage, path)
				return
			}
		}
	} else {
		for {
			balance := m.sizeLimitBytes - m.storageSizeBytes - len(img.Pix)
			if balance >= 0 {
				img2 := image.NewRGBA(img.Rect)
				copy(img2.Pix, img.Pix)
				ficsi := FileImageCacheStorageItemStruct{}
				ficsi.image = img2
				m.storage[path] = ficsi
				m.storageSizeBytes = balance
				m.badBalance = false
				return
			}
			m.badBalance = true
			if !m.removeUnpopularExclude(path) {
				return
			}
		}
	}
}

func (m *FileImageCacheStruct) Delete(path string) {
	delete(m.storage, path)
}

func (m *FileImageCacheStruct) removeUnpopularExclude(path string) bool {
	ok := false
	minK := ""
	minV := 0x7fffffffffffffff
	for k, v := range m.storage {
		if k != path {
			if v.loadCounter < minV {
				minK = k
				minV = v.loadCounter
				ok = true
			}
		}
	}
	if ok {
		size := len(m.storage[minK].image.Pix)
		delete(m.storage, minK)
		m.storageSizeBytes -= size
		return true
	}
	return false
}
