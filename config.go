package main

import (
	"encoding/json"
	"fmt"
	"os"
)

var Config ConfigStruct

type ConfigStruct struct {
	Keybindings ConfigKeybindings
	Common      ConfigCommon
}

type ConfigCommon struct {
	SaveMarkedToPersistent string `json:"SaveMarkedToPersistent"`
}

type ConfigKeybindings struct {
	Main          ConfigKeybindingsMain
	Markup        ConfigKeybindingsMarkup
	Window        ConfigKeybindingsWindow
	Help          ConfigKeybindingsHelp
	SaveNewMarked ConfigKeybindingsSaveNewMarked
}

type ConfigKeybindingsMain struct {
	Markup        string `json:"Markup"`
	Help          string `json:"Help"`
	Screenshot    string `json:"Screenshot"`
	Learn         string `json:"Learn"`
	Window        string `json:"Window"`
	SaveNewMarked string `json:"SaveNewMarked"`
}

type ConfigKeybindingsMarkup struct {
	SaveMarkup string `json:"Save markup"`
	Ignore     string `json:"Ignore"`
	Quit       string `json:"Quit"`
	Help       string `json:"Help"`
}

type ConfigKeybindingsWindow struct {
	Help string `json:"Help"`
	Quit string `json:"Quit"`
}

type ConfigKeybindingsSaveNewMarked struct {
	Quit string `json:"Quit"`
}

type ConfigKeybindingsHelp struct {
	Quit string `json:"Quit"`
}

func (c *ConfigStruct) Load() error {
	data, err := os.ReadFile("config.txt")
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, c)
	return err

}

func (c *ConfigStruct) Description() []string {
	a := make([]string, 0)
	a = append(a, "Main:")
	a = append(a, fmt.Sprintf("   Markup - %v", c.Keybindings.Main.Markup))
	a = append(a, fmt.Sprintf("   Make screenshot - %v", c.Keybindings.Main.Screenshot))
	a = append(a, fmt.Sprintf("   Learn - %v", c.Keybindings.Main.Learn))
	a = append(a, fmt.Sprintf("   Move new marked to persistent - %v", c.Keybindings.Main.SaveNewMarked))
	a = append(a, fmt.Sprintf("   Help - %v", c.Keybindings.Main.Help))
	a = append(a, fmt.Sprintf("   Select window - %v", c.Keybindings.Main.Window))
	a = append(a, "")
	a = append(a, "Markup:")
	a = append(a, fmt.Sprintf("   Save markup - %v", c.Keybindings.Markup.SaveMarkup))
	a = append(a, fmt.Sprintf("   Ignore - %v", c.Keybindings.Markup.SaveMarkup))
	a = append(a, fmt.Sprintf("   Help - %v", c.Keybindings.Markup.Help))
	a = append(a, fmt.Sprintf("   Quit - %v", c.Keybindings.Markup.Quit))
	a = append(a, "")
	a = append(a, "Select window:")
	a = append(a, fmt.Sprintf("   Help - %v", c.Keybindings.Window.Help))
	a = append(a, fmt.Sprintf("   Quit - %v", c.Keybindings.Window.Quit))
	a = append(a, "")
	a = append(a, "Move new marked to persistent:")
	a = append(a, fmt.Sprintf("   Quit - %v", c.Keybindings.SaveNewMarked.Quit))
	a = append(a, "")
	a = append(a, "Help:")
	a = append(a, fmt.Sprintf("   Quit - %v", c.Keybindings.Help.Quit))
	return a
}
