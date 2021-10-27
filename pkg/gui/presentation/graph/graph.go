package graph

import (
	"os"
	"sort"
	"time"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sirupsen/logrus"
)

type PipeKind uint8

const (
	TERMINATES PipeKind = iota
	STARTS
	CONTINUES
)

type Pipe struct {
	fromPos int
	toPos   int
	fromSha string
	toSha   string
	kind    PipeKind
	style   style.TextStyle
}

var highlightStyle = style.FgLightWhite.SetBold()

func ContainsCommitSha(pipes []Pipe, sha string) bool {
	for _, pipe := range pipes {
		if equalHashes(pipe.fromSha, sha) {
			return true
		}
	}
	return false
}

func (self Pipe) left() int {
	return utils.Min(self.fromPos, self.toPos)
}

func (self Pipe) right() int {
	return utils.Max(self.fromPos, self.toPos)
}

func (self Pipe) forSha(sha string) bool {
	return equalHashes(self.fromSha, sha) || equalHashes(self.toSha, sha)
}

func RenderCommitGraph(commits []*models.Commit, selectedCommit *models.Commit, getStyle func(c *models.Commit) style.TextStyle) ([][]Pipe, []string, int, int) {
	if len(commits) == 0 {
		return nil, nil, 0, 0
	}
	t := time.Now()

	pipes := []Pipe{{fromPos: 0, toPos: 0, fromSha: "START", toSha: commits[0].Sha, kind: STARTS, style: style.FgDefault}}

	pipeSets := [][]Pipe{}
	startOfSelection := -1
	endOfSelection := -1
	for _, commit := range commits {
		pipes = getNextPipes(pipes, commit, getStyle)
		pipeSets = append(pipeSets, pipes)
	}

	for i, pipeSet := range pipeSets {
		if ContainsCommitSha(pipeSet, selectedCommit.Sha) {
			if startOfSelection == -1 {
				startOfSelection = i
			}
			endOfSelection = i
		} else if endOfSelection != -1 {
			break
		}
	}

	lines := RenderAux(pipeSets, commits, selectedCommit.Sha)
	Log.Warn(t, time.Since(t))

	return pipeSets, lines, startOfSelection, endOfSelection
}

func RenderAux(pipeSets [][]Pipe, commits []*models.Commit, selectedCommitSha string) []string {
	lines := make([]string, 0, len(pipeSets))
	for i, pipeSet := range pipeSets {
		var prevCommit *models.Commit
		if i > 0 {
			prevCommit = commits[i-1]
		}
		cells := getCellsFromPipeSet(pipeSet, commits[i], selectedCommitSha, prevCommit)
		lines = append(lines, renderCells(cells))
	}
	return lines
}

func getNextPipes(prevPipes []Pipe, commit *models.Commit, getStyle func(c *models.Commit) style.TextStyle) []Pipe {
	currentPipes := []Pipe{}
	maxPos := 0
	for _, pipe := range prevPipes {
		if pipe.kind != TERMINATES {
			currentPipes = append(currentPipes, pipe)
		}
		maxPos = utils.Max(maxPos, pipe.toPos)
	}

	newPipes := []Pipe{}
	// TODO: need to decide on where to put the commit based on the leftmost pipe
	// that goes to the commit
	pos := -1
	for _, pipe := range currentPipes {
		if equalHashes(pipe.toSha, commit.Sha) {
			if pos == -1 {
				pos = pipe.toPos
			} else {
				pos = utils.Min(pipe.toPos, pos)
			}
		}
	}

	takenSpots := make(map[int]bool)
	traversedSpots := make(map[int]bool)
	if pos == -1 {
		pos = maxPos + 1
		takenSpots[pos] = true
		traversedSpots[pos] = true
	}

	newPipes = append(newPipes, Pipe{
		fromPos: pos,
		toPos:   pos,
		fromSha: commit.Sha,
		toSha:   commit.Parents[0],
		kind:    STARTS,
		style:   getStyle(commit),
	})

	otherMap := make(map[int]bool)
	for _, pipe := range currentPipes {
		if !equalHashes(pipe.toSha, commit.Sha) {
			otherMap[pipe.toPos] = true
		}
	}

	getNextAvailablePosForNewPipe := func() int {
		i := 0
		for {
			if !takenSpots[i] && !otherMap[i] {
				return i
			}
			i++
		}
	}

	getNextAvailablePos := func() int {
		i := 0
		for {
			if !traversedSpots[i] {
				return i
			}
			i++
		}
	}

	traverse := func(from, to int) {
		left, right := from, to
		if left > right {
			left, right = right, left
		}
		for i := left; i <= right; i++ {
			traversedSpots[i] = true
		}
		takenSpots[to] = true
	}

	for _, pipe := range currentPipes {
		if equalHashes(pipe.toSha, commit.Sha) {
			// terminating here
			newPipes = append(newPipes, Pipe{
				fromPos: pipe.toPos,
				toPos:   pos,
				fromSha: pipe.fromSha,
				toSha:   pipe.toSha,
				kind:    TERMINATES,
				style:   pipe.style,
			})
			traverse(pipe.toPos, pos)
		} else if pipe.toPos < pos {
			// continuing here
			availablePos := getNextAvailablePos()
			newPipes = append(newPipes, Pipe{
				fromPos: pipe.toPos,
				toPos:   availablePos,
				fromSha: pipe.fromSha,
				toSha:   pipe.toSha,
				kind:    CONTINUES,
				style:   pipe.style,
			})
			traverse(pipe.toPos, availablePos)
		}
	}

	if commit.IsMerge() {
		for _, parent := range commit.Parents[1:] {
			availablePos := getNextAvailablePosForNewPipe()
			// need to act as if continuing pipes are going to continue on the same line.
			newPipes = append(newPipes, Pipe{
				fromPos: pos,
				toPos:   availablePos,
				fromSha: commit.Sha,
				toSha:   parent,
				kind:    STARTS,
				style:   getStyle(commit),
			})

			takenSpots[availablePos] = true
		}
	}

	for _, pipe := range currentPipes {
		if !equalHashes(pipe.toSha, commit.Sha) && pipe.toPos > pos {
			// continuing on, potentially moving left to fill in a blank spot
			// actually need to work backwards: can't just fill any gap: or can I?
			last := pipe.toPos
			for i := pipe.toPos; i > pos; i-- {
				if takenSpots[i] || traversedSpots[i] {
					break
				} else {
					last = i
				}
			}
			newPipes = append(newPipes, Pipe{
				fromPos: pipe.toPos,
				toPos:   last,
				fromSha: pipe.fromSha,
				toSha:   pipe.toSha,
				kind:    CONTINUES,
				style:   pipe.style,
			})
			traverse(pipe.toPos, last)
		}
	}

	// not efficient but doing it for now: sorting my pipes by toPos, then by kind
	sort.Slice(newPipes, func(i, j int) bool {
		if newPipes[i].toPos == newPipes[j].toPos {
			return newPipes[i].kind < newPipes[j].kind
		}
		return newPipes[i].toPos < newPipes[j].toPos
	})

	return newPipes
}

func getCellsFromPipeSet(pipes []Pipe, commit *models.Commit, selectedCommitSha string, prevCommit *models.Commit) []*Cell {
	isMerge := commit.IsMerge()
	pos := 0
	for _, pipe := range pipes {
		if pipe.kind == STARTS {
			pos = pipe.fromPos
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

	renderPipe := func(pipe Pipe, style style.TextStyle, overrideRightStyle bool) {
		left := pipe.left()
		right := pipe.right()

		if left != right {
			for i := left + 1; i < right; i++ {
				cells[i].setLeft(style).setRight(style, overrideRightStyle)
			}
			cells[left].setRight(style, overrideRightStyle)
			cells[right].setLeft(style)
		}

		if pipe.kind == STARTS || pipe.kind == CONTINUES {
			cells[pipe.toPos].setDown(style)
		}
		if pipe.kind == TERMINATES || pipe.kind == CONTINUES {
			cells[pipe.fromPos].setUp(style)
		}
	}

	// we don't want to highlight two commits if they're contiguous. We only want
	// to highlight multiple things if there's an actual visible pipe involved.
	highlight := true
	if prevCommit != nil && equalHashes(prevCommit.Sha, selectedCommitSha) {
		highlight = false
		for _, pipe := range pipes {
			if equalHashes(pipe.fromSha, selectedCommitSha) && (pipe.kind != TERMINATES || pipe.fromPos != pipe.toPos) {
				highlight = true
			}
		}
	}

	// so we have our pos again, now it's time to build the cells.
	// we'll handle the one that's sourced from our selected commit last so that it can override the other cells.
	selectedPipes := []Pipe{}
	nonSelectedPipes := []Pipe{}

	for _, pipe := range pipes {
		if highlight && equalHashes(pipe.fromSha, selectedCommitSha) {
			selectedPipes = append(selectedPipes, pipe)
		} else {
			nonSelectedPipes = append(nonSelectedPipes, pipe)
		}
	}

	for _, pipe := range nonSelectedPipes {
		if pipe.kind == STARTS {
			renderPipe(pipe, pipe.style, true)
		}
	}

	for _, pipe := range nonSelectedPipes {
		if pipe.kind != STARTS && !(pipe.kind == TERMINATES && pipe.fromPos == pos && pipe.toPos == pos) {
			renderPipe(pipe, pipe.style, false)
		}
	}

	if len(selectedPipes) > 0 {
		for _, pipe := range selectedPipes {
			for i := pipe.left(); i <= pipe.right(); i++ {
				cells[i].reset()
			}
		}
		for _, pipe := range selectedPipes {
			renderPipe(pipe, highlightStyle, true)
			if pipe.toPos == pos {
				cells[pipe.toPos].setStyle(highlightStyle)
			}
		}
	}

	cType := COMMIT
	if isMerge {
		cType = MERGE
	}

	cells[pos].setType(cType)

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
