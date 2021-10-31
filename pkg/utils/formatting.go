package utils

import (
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/mattn/go-runewidth"
	"github.com/sirupsen/logrus"
)

// WithPadding pads a string as much as you want
func WithPadding(str string, padding int) string {
	uncoloredStr := Decolorise(str)
	width := runewidth.StringWidth(uncoloredStr)
	if padding < width {
		return str
	}
	return str + strings.Repeat(" ", padding-width)
}

func RenderDisplayStrings(displayStringsArr [][]string) string {
	padWidths := getPadWidths(displayStringsArr)
	output := getPaddedDisplayStrings(displayStringsArr, padWidths)

	return output
}

func getPaddedDisplayStrings(stringArrays [][]string, padWidths []int) string {
	builder := strings.Builder{}
	for i, stringArray := range stringArrays {
		if len(stringArray) == 0 {
			continue
		}
		for j, padWidth := range padWidths {
			if len(stringArray)-1 < j {
				continue
			}
			builder.WriteString(WithPadding(stringArray[j], padWidth))
			builder.WriteString(" ")
		}
		if len(stringArray)-1 < len(padWidths) {
			continue
		}
		builder.WriteString(stringArray[len(padWidths)])

		if i < len(stringArrays)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func getPadWidths(stringArrays [][]string) []int {
	maxWidth := 0
	for _, stringArray := range stringArrays {
		if len(stringArray) > maxWidth {
			maxWidth = len(stringArray)
		}
	}
	if maxWidth-1 < 0 {
		return []int{}
	}
	padWidths := make([]int, maxWidth-1)
	for i := range padWidths {
		for _, strings := range stringArrays {
			if len(strings) == 0 {
				panic(spew.Sdump(stringArrays))
			}
			uncoloredStr := Decolorise(strings[i])

			width := runewidth.StringWidth(uncoloredStr)
			if width > padWidths[i] {
				padWidths[i] = width
			}
		}
	}
	return padWidths
}

// TruncateWithEllipsis returns a string, truncated to a certain length, with an ellipsis
func TruncateWithEllipsis(str string, limit int) string {
	if runewidth.StringWidth(str) > limit && limit <= 3 {
		return strings.Repeat(".", limit)
	}
	return runewidth.Truncate(str, limit, "...")
}

func SafeTruncate(str string, limit int) string {
	if len(str) > limit {
		return str[0:limit]
	} else {
		return str
	}
}

func newLogger() *logrus.Entry {
	logPath := "/Users/jesseduffieldduffield/Library/Application Support/jesseduffield/lazygit/development.log"
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("unable to log to file") // TODO: don't panic (also, remove this call to the `panic` function)
	}
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	logger.SetOutput(file)
	return logger.WithFields(logrus.Fields{})
}

var Log = newLogger()
