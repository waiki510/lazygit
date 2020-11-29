package presentation

import (
	"github.com/fatih/color"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/theme"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

func GetFileListDisplayStrings(files []*models.File, diffName string, submoduleConfigs []*models.SubmoduleConfig, selectedFilenames map[string]bool) [][]string {
	lines := make([][]string, len(files))

	for i, file := range files {
		diffed := file.Name == diffName
		isSelected := selectedFilenames[file.Name]
		lines[i] = getFileDisplayStrings(file, diffed, submoduleConfigs, isSelected)
	}

	return lines
}

// getFileDisplayStrings returns the display string of branch
func getFileDisplayStrings(f *models.File, diffed bool, submoduleConfigs []*models.SubmoduleConfig, isSelected bool) []string {
	// potentially inefficient to be instantiating these color
	// objects with each render
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	diffColor := color.New(theme.DiffTerminalColor)

	var restColor *color.Color
	if diffed {
		restColor = diffColor
	} else if f.HasUnstagedChanges {
		restColor = red
	} else {
		restColor = green
	}

	// this is just making things look nice when the background attribute is 'reverse'
	firstChar := f.DisplayString[0:1]
	firstCharCl := green
	if firstChar == " " || firstChar == "?" {
		firstCharCl = restColor
	}

	secondChar := f.DisplayString[1:2]
	secondCharCl := red
	if secondChar == " " {
		secondCharCl = restColor
	}

	if isSelected {
		firstCharCl.Add(theme.SelectedRangeBgColor)
		secondCharCl.Add(theme.SelectedRangeBgColor)
		restColor.Add(theme.SelectedRangeBgColor)
	}

	output := firstCharCl.Sprint(firstChar)
	output += secondCharCl.Sprint(secondChar)
	output += restColor.Sprintf(" %s", f.Name)

	if f.IsSubmodule(submoduleConfigs) {
		output += utils.ColoredString(" (submodule)", theme.DefaultTextColor)
	}

	return []string{output}
}
