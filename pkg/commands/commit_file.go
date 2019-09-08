package commands

import "github.com/fatih/color"

// CommitFile : A git commit file
type CommitFile struct {
	Sha           string
	Name          string
	DisplayString string
	Highlighted   bool
}

// GetDisplayStrings is a function.
func (f *CommitFile) GetDisplayStrings(isFocused bool) []string {
	var commitColor *color.Color
	if f.Highlighted {
		commitColor = color.New(color.FgCyan, color.BgBlue)
	} else {
		commitColor = color.New(color.FgWhite)
	}

	return []string{commitColor.Sprint(f.DisplayString)}
}
