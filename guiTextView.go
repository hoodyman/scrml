package main

import "sync"

type GuiTextViewStruct struct {
	mut     sync.Mutex
	lineCap int
	lines   []string
}

var GuiTextView GuiTextViewStruct

func (m *GuiTextViewStruct) SetNumLines(linesNum int) {
	m.mut.Lock()
	if linesNum < 1 {
		m.lineCap = 0
	} else {
		m.lineCap = linesNum
	}
	m.mut.Unlock()
}

func (m *GuiTextViewStruct) PutString(s string) {
	m.mut.Lock()
	if m.lineCap == 0 {
		return
	}
	if len(m.lines) == m.lineCap {
		m.lines = m.lines[1:]
	}
	m.lines = append(m.lines, s)
	m.mut.Unlock()
}

func (m *GuiTextViewStruct) Clean() {
	m.mut.Lock()
	m.lines = make([]string, 0, m.lineCap)
	m.mut.Unlock()
}

func (m *GuiTextViewStruct) GetLines() []string {
	m.mut.Lock()
	l := make([]string, len(m.lines))
	copy(l, m.lines)
	m.mut.Unlock()
	return l
}
