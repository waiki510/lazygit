package presentation

import (
	"os"
	"strings"
	"sync"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/authors"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation/graph"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/theme"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/kyokomi/emoji/v2"
	"github.com/sirupsen/logrus"
)

type pipeSetCacheKey struct {
	commitSha   string
	commitCount int
}

var pipeSetCache map[pipeSetCacheKey][][]graph.Pipe
var mutex sync.Mutex

func getCommitListDisplayStrings(
	commits []*models.Commit,
	graphLines []string,
	fullDescription bool,
	cherryPickedCommitShaMap map[string]bool,
	diffName string,
	parseEmoji bool,
) [][]string {
	lines := make([][]string, 0, len(graphLines))
	for i, commit := range commits {
		lines = append(lines, displayCommit(commit, cherryPickedCommitShaMap, diffName, parseEmoji, graphLines[i], fullDescription))
	}
	return lines
}

func GetCommitListDisplayStrings(
	commits []*models.Commit,
	fullDescription bool,
	cherryPickedCommitShaMap map[string]bool,
	diffName string,
	parseEmoji bool,
	selectedCommit *models.Commit,
	startIdx int,
	length int,
) [][]string {
	mutex.Lock()
	defer mutex.Unlock()

	if len(commits) == 0 {
		return nil
	}

	// given that our cache key is a commit sha and a commit count, it's very important that we don't actually try to render pipes
	// when dealing with things like filtered commits.
	cacheKey := pipeSetCacheKey{
		commitSha:   commits[0].Sha,
		commitCount: len(commits),
	}

	if pipeSetCache == nil {
		pipeSetCache = make(map[pipeSetCacheKey][][]graph.Pipe)
	}

	pipeSets, ok := pipeSetCache[cacheKey]
	if !ok {
		// pipe sets are unique to a commit head. and a commit count. Sometimes we haven't loaded everything for that.
		// so let's just cache it based on that.
		getStyle := func(commit *models.Commit) style.TextStyle {
			return authors.AuthorStyle(commit.Author)
		}
		pipeSets = graph.GetPipeSets(commits, selectedCommit, getStyle)
		pipeSetCache[cacheKey] = pipeSets
	}

	if len(commits) == 0 {
		return [][]string{}
	}

	end := startIdx + length
	if end > len(commits)-1 {
		end = len(commits) - 1
	}

	filteredPipeSets := pipeSets[startIdx : end+1]
	filteredCommits := commits[startIdx : end+1]
	graphLines := graph.RenderAux(filteredPipeSets, filteredCommits, selectedCommit.Sha)
	return getCommitListDisplayStrings(
		commits[startIdx:end+1],
		graphLines,
		fullDescription,
		cherryPickedCommitShaMap,
		diffName,
		parseEmoji,
	)
}

func displayCommit(
	commit *models.Commit,
	cherryPickedCommitShaMap map[string]bool,
	diffName string,
	parseEmoji bool,
	graphLine string,
	fullDescription bool,
) []string {

	shaColor := getShaColor(commit, diffName, cherryPickedCommitShaMap)

	actionString := ""
	if commit.Action != "" {
		actionString = actionColorMap(commit.Action).Sprint(commit.Action) + " "
	}

	tagString := ""
	if fullDescription {
		if len(commit.Tags) > 0 {
			tagString = theme.DiffTerminalColor.SetBold().Sprint(strings.Join(commit.Tags, " ")) + " "
		}
	} else {
		if commit.ExtraInfo != "" {
			tagString = style.FgMagenta.SetBold().Sprint(commit.ExtraInfo) + " "
		}
	}

	name := commit.Name
	if parseEmoji {
		name = emoji.Sprint(name)
	}

	authorFunc := authors.ShortAuthor
	if fullDescription {
		authorFunc = authors.LongAuthor
	}

	cols := make([]string, 0, 5)
	cols = append(cols, shaColor.Sprint(commit.ShortSha()))
	if fullDescription {
		cols = append(cols, style.FgBlue.Sprint(utils.UnixToDate(commit.UnixTimestamp)))
	}
	cols = append(
		cols,
		actionString,
		authorFunc(commit.Author),
		graphLine+tagString+theme.DefaultTextColor.Sprint(name),
	)

	return cols

}

func getShaColor(commit *models.Commit, diffName string, cherryPickedCommitShaMap map[string]bool) style.TextStyle {
	diffed := commit.Sha == diffName
	shaColor := theme.DefaultTextColor
	switch commit.Status {
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
	} else if cherryPickedCommitShaMap[commit.Sha] {
		shaColor = theme.CherryPickedCommitTextStyle
	}

	return shaColor
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
