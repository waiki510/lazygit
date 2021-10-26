package graph

import (
	"os"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Line struct {
	IsHighlighted bool
	Content       string
}

type PipeSet struct {
	pipes   []Pipe
	isMerge bool
}

func (self PipeSet) ContainsCommitSha(sha string) bool {
	for _, pipe := range self.pipes {
		if equalHashes(pipe.sourceCommitSha, sha) {
			return true
		}
	}
	return false
}

type Path struct {
	from       string
	to         string
	styleIndex int
}

func (p Path) id() string {
	// from will be unique per path
	return p.from
}

type PipeKind uint8

const (
	STARTS PipeKind = iota
	TERMINATES
	CONTINUES
)

type Pipe struct {
	fromPos         int
	toPos           int
	kind            PipeKind
	style           style.TextStyle
	sourceCommitSha string
}

func (self Pipe) left() int {
	return utils.Min(self.fromPos, self.toPos)
}

func (self Pipe) right() int {
	return utils.Max(self.fromPos, self.toPos)
}

var styles = []style.TextStyle{
	style.FgDefault,
	style.FgCyan,
	style.FgBlue,
	style.FgGreen,
	style.FgYellow,
	style.FgMagenta,
	style.FgRed,
}

func getNextStyleIndex(index int) int {
	if index == len(styles)-1 {
		return 0
	} else {
		return index + 1
	}
}

func getNextStyleIndexForPath(paths []Path) int {
	if len(paths) == 0 {
		return 0
	}

	return getNextStyleIndex(paths[len(paths)-1].styleIndex)
}

func getStyle(index int) style.TextStyle {
	return styles[index]
}

func RenderCommitGraph(commits []*models.Commit, selectedCommit *models.Commit) ([]PipeSet, []string, int, int) {
	if len(commits) == 0 {
		return nil, nil, 0, 0
	}

	// arbitrarily capping capacity at 20
	paths := make([]Path, 0, 20)
	paths = append(paths, Path{from: "START", to: commits[0].Sha, styleIndex: getNextStyleIndexForPath(paths)})

	// arbitrarily capping capacity at 20
	pipeSets := []PipeSet{}
	startOfSelection := -1
	endOfSelection := -1
	for _, commit := range commits {
		nextPaths := getNextPaths(paths, commit)
		pipes := getPipesFromPaths(paths, nextPaths, commit.Sha)
		paths = nextPaths
		pipeSets = append(pipeSets, PipeSet{pipes: pipes, isMerge: commit.IsMerge()})
	}

	for i, pipeSet := range pipeSets {
		if pipeSet.ContainsCommitSha((selectedCommit.Sha)) {
			if startOfSelection == -1 {
				startOfSelection = i
			}
			endOfSelection = i
		} else if endOfSelection != -1 {
			break
		}
	}

	lines := RenderAux(pipeSets, selectedCommit.Sha)

	return pipeSets, lines, startOfSelection, endOfSelection
}

func RenderAux(pipeSets []PipeSet, selectedCommitSha string) []string {
	lines := make([]string, 0, len(pipeSets))
	for _, pipeSet := range pipeSets {
		cells := getCellsFromPipeSet(pipeSet, selectedCommitSha)
		line := ""
		for _, cell := range cells {
			line += cell.render()
		}
		lines = append(lines, line)
	}
	return lines
}

func getNextPaths(paths []Path, commit *models.Commit) []Path {
	pos := -1
	for i, path := range paths {
		if equalHashes(path.to, commit.Sha) {
			if pos == -1 {
				pos = i
			}
		}
	}

	nextColour := getNextStyleIndexForPath(paths)

	// fmt.Println("commit: " + commit.Sha)
	// fmt.Println("paths")
	// fmt.Println(spew.Sdump(paths))

	newPaths := make([]Path, 0, len(paths))
	for i, path := range paths {
		if path.from == "GHOST" {
			continue
		}
		if equalHashes(path.to, commit.Sha) {
			if i == pos {
				newPaths = append(newPaths, Path{from: commit.Sha, to: commit.Parents[0], styleIndex: nextColour})
			} else {
				newPaths = append(newPaths, Path{from: "GHOST", to: "GHOST", styleIndex: nextColour})
			}
		} else {
			newPaths = append(newPaths, path)
			for j := len(newPaths); j < i+1; j++ {
				newPaths = append(newPaths, Path{from: "GHOST", to: "GHOST", styleIndex: nextColour})
			}
		}
	}

	if pos == -1 {
		newPaths = append(newPaths, Path{from: commit.Sha, to: commit.Parents[0], styleIndex: nextColour})
	}

	if commit.IsMerge() {
	outer:
		for _, parentSha := range commit.Parents[1:] {
			// we are allowed to overwrite a ghost here
			for i, path := range newPaths {
				if path.from == "GHOST" {
					// fmt.Println("replacing ghost for " + parentSha)
					newPaths[i] = Path{from: commit.Sha, to: parentSha, styleIndex: nextColour}
					continue outer
				}
			}
			newPaths = append(newPaths, Path{from: commit.Sha, to: parentSha, styleIndex: nextColour})
		}
	}

	// fmt.Println("new paths")
	// fmt.Println(spew.Sdump(newPaths))

	return newPaths
}

func getPipesFromPaths(beforePaths []Path, afterPaths []Path, commitSha string) []Pipe {
	// we can never add more than one pipe at a time
	pipes := make([]Pipe, 0, len(beforePaths)+1)

	matched := map[string]bool{}

	commitPos := -1
	for i, path := range beforePaths {
		if equalHashes(path.to, commitSha) {
			commitPos = i
			break
		}
	}
	if commitPos == -1 {
		for i, path := range afterPaths {
			if equalHashes(path.from, commitSha) {
				commitPos = i
				break
			}
		}
	}

	key := func(path Path) string {
		return path.from + "-" + path.to
	}

outer:
	for i, beforePath := range beforePaths {
		if beforePath.from == "GHOST" {
			continue
		}
		for j, afterPath := range afterPaths {
			if afterPath.from == "GHOST" {
				continue
			}
			if beforePath.from == afterPath.from && beforePath.to == afterPath.to {
				// see if I can honour this line being blocked by other lines
				pipes = append(pipes, Pipe{fromPos: i, toPos: j, kind: CONTINUES, style: getStyle(beforePath.styleIndex), sourceCommitSha: beforePath.from})
				matched[key(afterPath)] = true
				continue outer
			}
		}
		pipes = append(pipes, Pipe{fromPos: i, toPos: commitPos, kind: TERMINATES, style: getStyle(beforePath.styleIndex), sourceCommitSha: beforePath.from})
	}

	for i, afterPath := range afterPaths {
		if afterPath.from == "GHOST" {
			continue
		}
		if !matched[key(afterPath)] {
			pipes = append(pipes, Pipe{fromPos: commitPos, toPos: i, kind: STARTS, style: getStyle(afterPath.styleIndex), sourceCommitSha: afterPath.from})
		}
	}

	return pipes
}

func getCellsFromPipeSet(pipeSet PipeSet, selectedCommitSha string) []*Cell {
	pipes := pipeSet.pipes
	isMerge := pipeSet.isMerge

	pos := 0
	var sourcePipe Pipe
	for _, pipe := range pipes {
		if pipe.kind == STARTS {
			pos = pipe.fromPos
			sourcePipe = pipe
		} else if pipe.kind == TERMINATES {
			pos = pipe.toPos
		}
	}

	maxPos := 0
	for _, pipe := range pipes {
		if pipe.right() > maxPos {
			maxPos = pipe.right()
		}
	}
	cells := make([]*Cell, maxPos+1)
	for i := range cells {
		cells[i] = &Cell{cellType: CONNECTION, style: style.FgDefault}
	}

	renderPipe := func(pipe Pipe, style style.TextStyle) {
		left := pipe.left()
		right := pipe.right()

		if left != right {
			for i := left + 1; i < right; i++ {
				cells[i].setLeft(style).setRight(style)
			}
			cells[left].setRight(style)
			cells[right].setLeft(style)
		}

		if pipe.kind == STARTS || pipe.kind == CONTINUES {
			cells[pipe.toPos].setDown(style)
		}
		if pipe.kind == TERMINATES || pipe.kind == CONTINUES {
			cells[pipe.fromPos].setUp(style)
		}
	}

	// so we have our pos again, now it's time to build the cells.
	// we'll handle the one that's sourced from our selected commit last so that it can override the other cells.
	selectedPipes := []Pipe{}

	selectionCount := 0
	for _, pipe := range pipes {
		if equalHashes(pipe.sourceCommitSha, selectedCommitSha) {
			selectionCount++
		}
	}

	for _, pipe := range pipes {
		if equalHashes(pipe.sourceCommitSha, selectedCommitSha) {
			selectedPipes = append(selectedPipes, pipe)
		} else {
			s := pipe.style
			if selectionCount > 2 {
				s = style.FgBlack
			}
			renderPipe(pipe, s)
		}
	}

	if len(selectedPipes) > 0 {
		for _, pipe := range selectedPipes {
			for i := pipe.left(); i <= pipe.right(); i++ {
				cells[i].reset()
			}
		}
		for _, pipe := range selectedPipes {
			renderPipe(pipe, pipe.style.SetBold())
		}
	}

	// commitSelected := false
	// for _, pipe := range pipes {
	// 	// if pipe.sourceCommitSha
	// }

	cType := COMMIT
	if isMerge {
		cType = MERGE
	}

	cells[pos].setType(cType)
	if selectionCount > 1 {
		cells[pos].setStyle(sourcePipe.style)
	}

	return cells
}

func equalHashes(a, b string) bool {
	length := utils.Min(len(a), len(b))
	// parent hashes are only stored up to 20 characters for some reason so we'll truncate to that for comparison
	return a[:length] == b[:length]
}

// instead of taking the previous path array and rendering the current line, we take the previous and next path arrays and render the current line.

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
