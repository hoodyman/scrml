package main

type ToolsStruct struct{}

var Tools ToolsStruct

func (m *ToolsStruct) GetMapStringByteFirstElement(v *map[string]byte) (string, bool) {
	for k := range *v {
		return k, true
	}
	return "", false
}
