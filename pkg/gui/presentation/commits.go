package presentation

import (
	"strings"
	"sync"

	"crypto/md5"
	"os"
	"strings"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/authors"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/graph"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/theme"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/kyokomi/emoji/v2"
	"github.com/sirupsen/logrus"
)

var lastPipeSets []graph.PipeSet
var OldStart int = -1
var OldEnd int = -1
var mutex sync.Mutex

func ResetOldCommitLines(
	commits []*models.Commit,
	fullDescription bool,
	cherryPickedCommitShaMap map[string]bool,
	diffName string,
	parseEmoji bool,
	selectedCommit *models.Commit,
) [][]string {
	mutex.Lock()
	defer mutex.Unlock()
	lines := [][]string{}

	var displayFunc func(*models.Commit, map[string]bool, bool, bool, string) []string
	if fullDescription {
		displayFunc = getFullDescriptionDisplayStringsForCommit
	} else {
		displayFunc = getDisplayStringsForCommit
	}

	// all I need to do is reconstruct the graph lines for the old commit and the newly selected commit. It would be good if I had a history of the range of the last selected one.

	if OldStart == -1 || OldEnd == -1 {
		return [][]string{}
	}

	// TODO: off by one error?
	graphLines := graph.RenderAux(lastPipeSets[OldStart:OldEnd+1], selectedCommit.Sha)

	// need to make use of lastBlahs
	for i := OldStart; i <= OldEnd && i < len(commits); i++ {
		commit := commits[i]
		diffed := commit.Sha == diffName
		lines = append(lines, displayFunc(commit, cherryPickedCommitShaMap, diffed, parseEmoji, graphLines[i-OldStart]))
	}

	return lines
}

func SetNewSelection(
	commits []*models.Commit,
	fullDescription bool,
	cherryPickedCommitShaMap map[string]bool,
	diffName string,
	parseEmoji bool,
	selectedCommit *models.Commit,
	index int,
) [][]string {
	mutex.Lock()
	defer mutex.Unlock()
	lines := [][]string{}

	var displayFunc func(*models.Commit, map[string]bool, bool, bool, string) []string
	if fullDescription {
		displayFunc = getFullDescriptionDisplayStringsForCommit
	} else {
		displayFunc = getDisplayStringsForCommit
	}

	end := index
	for i := index; i < len(commits); i++ {
		if !lastPipeSets[i].ContainsCommitSha(selectedCommit.Sha) {
			end = i - 1
			break
		}
	}

	graphLines := graph.RenderAux(lastPipeSets[index:end+1], selectedCommit.Sha)

	for i := index; i <= end; i++ {
		commit := commits[i]
		diffed := commit.Sha == diffName
		lines = append(lines, displayFunc(commit, cherryPickedCommitShaMap, diffed, parseEmoji, graphLines[i-index]))
	}

	OldStart = index
	OldEnd = end
	return lines
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

func GetCommitListDisplayStrings(commits []*models.Commit, fullDescription bool, cherryPickedCommitShaMap map[string]bool, diffName string, parseEmoji bool, selectedCommit *models.Commit) [][]string {
	mutex.Lock()
	defer mutex.Unlock()
	lines := make([][]string, len(commits))

	var displayFunc func(*models.Commit, map[string]bool, bool, bool, string) []string
	if fullDescription {
		displayFunc = getFullDescriptionDisplayStringsForCommit
	} else {
		displayFunc = getDisplayStringsForCommit
	}

	pipeSets, graphLines, start, end := graph.RenderCommitGraph(commits, selectedCommit)
	Log.Warn(spew.Sdump(graphLines[0:10]))
	lastPipeSets = pipeSets
	OldStart = start
	OldEnd = end

	for i, commit := range commits {
		diffed := commit.Sha == diffName
		lines[i] = displayFunc(commit, cherryPickedCommitShaMap, diffed, parseEmoji, graphLines[i])
	}

	return lines
}

func getFullDescriptionDisplayStringsForCommit(c *models.Commit, cherryPickedCommitShaMap map[string]bool, diffed, parseEmoji bool, graphLine string) []string {
	shaColor := theme.DefaultTextColor
	switch c.Status {
	case "unpushed":
		shaColor = style.FgRed
	case "pushed":
		shaColor = style.FgYellow
	case "merged":
		shaColor = style.FgGreen
	case "rebasing":
		shaColor = style.FgBlue
	case "reflog":
		shaColor = style.FgBlue
	}

	if diffed {
		shaColor = theme.DiffTerminalColor
	} else if cherryPickedCommitShaMap[c.Sha] {
		// for some reason, setting the background to blue pads out the other commits
		// horizontally. For the sake of accessibility I'm considering this a feature,
		// not a bug
		shaColor = theme.CherryPickedCommitTextStyle
	}

	tagString := ""
	secondColumnString := ""
	if c.Action != "" {
		secondColumnString = actionColorMap(c.Action).Sprint(c.Action)
	} else if c.ExtraInfo != "" {
		tagString = style.FgMagenta.SetBold().Sprint(c.ExtraInfo) + " "
	}

	name := c.Name
	if parseEmoji {
		name = emoji.Sprint(name)
	}

	Log.Warn(authors.LongAuthor(c.Author))

	return []string{
		shaColor.Sprint(c.ShortSha()),
		secondColumnString,
		authors.LongAuthor(c.Author),
		graphLine + tagString + theme.DefaultTextColor.Sprint(name),
	}
}

func getDisplayStringsForCommit(c *models.Commit, cherryPickedCommitShaMap map[string]bool, diffed, parseEmoji bool, graphLine string) []string {
	shaColor := theme.DefaultTextColor
	switch c.Status {
	case "unpushed":
		shaColor = style.FgRed
	case "pushed":
		shaColor = style.FgYellow
	case "merged":
		shaColor = style.FgGreen
	case "rebasing":
		shaColor = style.FgBlue
	case "reflog":
		shaColor = style.FgBlue
	}

	if diffed {
		shaColor = theme.DiffTerminalColor
	} else if cherryPickedCommitShaMap[c.Sha] {
		// for some reason, setting the background to blue pads out the other commits
		// horizontally. For the sake of accessibility I'm considering this a feature,
		// not a bug
		shaColor = theme.CherryPickedCommitTextStyle
	}

	actionString := ""
	tagString := ""
	if c.Action != "" {
		actionString = actionColorMap(c.Action).Sprint(utils.WithPadding(c.Action, 7)) + " "
	} else if len(c.Tags) > 0 {
		tagString = theme.DiffTerminalColor.SetBold().Sprint(strings.Join(c.Tags, " ")) + " "
	}

	name := c.Name
	if parseEmoji {
		name = emoji.Sprint(name)
	}

	Log.Warn(authors.ShortAuthor(c.Author))

	return []string{
		shaColor.Sprint(c.ShortSha()),
		authors.ShortAuthor(c.Author),
		graphLine + actionString + tagString + theme.DefaultTextColor.Sprint(name),
	}
}

func actionColorMap(str string) style.TextStyle {
	switch str {
	case "pick":
		return style.FgCyan
	case "drop":
		return style.FgRed
	case "edit":
		return style.FgGreen
	case "fixup":
		return style.FgMagenta
	default:
		return style.FgYellow
	}
}
