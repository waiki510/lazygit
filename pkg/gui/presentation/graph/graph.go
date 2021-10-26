package graph

import (
	"os"
	"sort"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Line struct {
	IsHighlighted bool
	Content       string
}

func ContainsCommitSha(pipes []Pipe, sha string) bool {
	for _, pipe := range pipes {
		if equalHashes(pipe.fromSha, sha) {
			return true
		}
	}
	return false
}

type Path struct {
	from  string
	to    string
	style style.TextStyle
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
	fromPos int
	toPos   int
	fromSha string
	toSha   string
	kind    PipeKind
	// style   style.TextStyle
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

func RenderCommitGraph(commits []*models.Commit, selectedCommit *models.Commit) ([][]Pipe, []string, int, int) {
	if len(commits) == 0 {
		return nil, nil, 0, 0
	}

	pipes := []Pipe{{fromPos: 0, toPos: 0, fromSha: "START", toSha: commits[0].Sha, kind: STARTS}}

	pipeSets := [][]Pipe{}
	startOfSelection := -1
	endOfSelection := -1
	for _, commit := range commits {
		pipes = getNextPipes(pipes, commit)
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

	return pipeSets, lines, startOfSelection, endOfSelection
}

func RenderAux(pipeSets [][]Pipe, commits []*models.Commit, selectedCommitSha string) []string {
	lines := make([]string, 0, len(pipeSets))
	for i, pipeSet := range pipeSets {
		cells := getCellsFromPipeSet(pipeSet, commits[i], selectedCommitSha)
		line := ""
		for _, cell := range cells {
			line += cell.render()
		}
		lines = append(lines, line)
	}
	return lines
}

func getNextPipes(prevPipes []Pipe, commit *models.Commit) []Pipe {
	// should potentially sort by toPos beforehand

	currentPipes := []Pipe{}
	for _, pipe := range prevPipes {
		if pipe.kind != TERMINATES {
			currentPipes = append(currentPipes, pipe)
		}
	}

	newPipes := []Pipe{}
	// need to decide on where to put the commit based on the leftmost pipe
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
	if pos != -1 {
		takenSpots[pos] = true
		traversedSpots[pos] = true
		newPipes = append(newPipes, Pipe{
			fromPos: pos,
			toPos:   pos,
			fromSha: commit.Sha,
			toSha:   commit.Parents[0],
			kind:    STARTS,
		})
	} else {
		Log.Warn("pos is -1")
	}

	// TODO: deal with newly added commit

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
			})
			traverse(pipe.fromPos, pos)
		} else if pipe.toPos < pos {
			// continuing here
			availablePos := getNextAvailablePos()
			newPipes = append(newPipes, Pipe{
				fromPos: pipe.toPos,
				toPos:   availablePos,
				fromSha: pipe.fromSha,
				toSha:   pipe.toSha,
				kind:    CONTINUES,
			})
			traverse(pipe.fromPos, availablePos)
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
				if !takenSpots[i] && !traversedSpots[i] {
					last = i
				}
			}
			newPipes = append(newPipes, Pipe{
				fromPos: pipe.toPos,
				toPos:   last,
				fromSha: pipe.fromSha,
				toSha:   pipe.toSha,
				kind:    CONTINUES,
			})
			traverse(pipe.fromPos, last)
		}
	}

	// not efficient but doing it for now: sorting my pipes by toPos
	sort.Slice(newPipes, func(i, j int) bool {
		return newPipes[i].toPos < newPipes[j].toPos
	})

	return newPipes
}

// func getNextPaths(paths []Path, commit *models.Commit) []Path {
// 	pos := -1
// 	for i, path := range paths {
// 		if equalHashes(path.to, commit.Sha) {
// 			if pos == -1 {
// 				pos = i
// 			}
// 		}
// 	}

// 	nextColour := authors.AuthorStyle(commit.Author)

// 	// fmt.Println("commit: " + commit.Sha)
// 	// fmt.Println("paths")
// 	// fmt.Println(spew.Sdump(paths))

// 	newPaths := make([]Path, 0, len(paths))
// 	for _, path := range paths {
// 		newPaths = append(newPaths, path)
// 	}

// 	if pos == -1 {
// 		newPaths = append(newPaths, Path{from: commit.Sha, to: commit.Parents[0], style: nextColour})
// 	} else {
// 		newPaths[pos] = Path{from: commit.Sha, to: commit.Parents[0], style: nextColour}
// 	}

// 	if commit.IsMerge() {
// 	outer:
// 		for _, parentSha := range commit.Parents[1:] {
// 			// we are allowed to overwrite a ghost here
// 			for i, path := range newPaths {
// 				if path.from == "GHOST" {
// 					// fmt.Println("replacing ghost for " + parentSha)
// 					newPaths[i] = Path{from: commit.Sha, to: parentSha, style: nextColour}
// 					continue outer
// 				}
// 			}
// 			newPaths = append(newPaths, Path{from: commit.Sha, to: parentSha, style: nextColour})
// 		}
// 	}

// 	for i, path := range newPaths {
// 		if path.from == "GHOST" {
// 			continue
// 		}
// 		if equalHashes(path.to, commit.Sha) {
// 			newPaths[i] = Path{from: "GHOST", to: "GHOST", style: nextColour}
// 		} else {
// 			newPaths[i] = path
// 			for j := len(newPaths); j < i+1; j++ {
// 				newPaths = append(newPaths, Path{from: "GHOST", to: "GHOST", style: nextColour})
// 			}
// 		}
// 	}

// 	// fmt.Println("new paths")
// 	// fmt.Println(spew.Sdump(newPaths))

// 	return newPaths
// }

// func getPipesFromPaths(beforePaths []Path, afterPaths []Path, commitSha string) []Pipe {
// 	// we can never add more than one pipe at a time
// 	pipes := make([]Pipe, 0, len(beforePaths)+1)

// 	matched := map[string]bool{}

// 	commitPos := -1
// 	for i, path := range beforePaths {
// 		if equalHashes(path.to, commitSha) {
// 			commitPos = i
// 			break
// 		}
// 	}
// 	if commitPos == -1 {
// 		for i, path := range afterPaths {
// 			if equalHashes(path.from, commitSha) {
// 				commitPos = i
// 				break
// 			}
// 		}
// 	}

// 	key := func(path Path) string {
// 		return path.from + "-" + path.to
// 	}

// outer:
// 	for i, beforePath := range beforePaths {
// 		if beforePath.from == "GHOST" {
// 			continue
// 		}
// 		for j, afterPath := range afterPaths {
// 			if afterPath.from == "GHOST" {
// 				continue
// 			}
// 			if beforePath.from == afterPath.from && beforePath.to == afterPath.to {
// 				// see if I can honour this line being blocked by other lines
// 				pipes = append(pipes, Pipe{fromPos: i, toPos: j, kind: CONTINUES, style: beforePath.style, sourceCommitSha: beforePath.from})
// 				matched[key(afterPath)] = true
// 				continue outer
// 			}
// 		}
// 		pipes = append(pipes, Pipe{fromPos: i, toPos: commitPos, kind: TERMINATES, style: beforePath.style, sourceCommitSha: beforePath.from})
// 	}

// 	for i, afterPath := range afterPaths {
// 		if afterPath.from == "GHOST" {
// 			continue
// 		}
// 		if !matched[key(afterPath)] {
// 			pipes = append(pipes, Pipe{fromPos: commitPos, toPos: i, kind: STARTS, style: afterPath.style, sourceCommitSha: afterPath.from})
// 		}
// 	}

// 	return pipes
// }

func getCellsFromPipeSet(pipes []Pipe, commit *models.Commit, selectedCommitSha string) []*Cell {
	isMerge := commit.IsMerge()

	pos := 0
	// var sourcePipe Pipe
	for _, pipe := range pipes {
		if pipe.kind == STARTS {
			pos = pipe.fromPos
			// sourcePipe = pipe
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
		if equalHashes(pipe.fromSha, selectedCommitSha) {
			selectionCount++
		}
	}

	for _, pipe := range pipes {
		if equalHashes(pipe.fromSha, selectedCommitSha) {
			selectedPipes = append(selectedPipes, pipe)
		} else {
			// TODO: add dynamic styling by passing in sha-style mapping
			s := style.FgDefault
			if selectionCount > 2 {
				s = style.FgBlack
			}
			renderPipe(pipe, s)
		}
	}

	if len(selectedPipes) > 0 {
		for _, pipe := range selectedPipes {
			Log.Warn(pipe.left())
			for i := pipe.left(); i <= pipe.right(); i++ {
				cells[i].reset()
			}
		}
		for _, pipe := range selectedPipes {
			renderPipe(pipe, style.FgLightWhite.SetBold())
		}
	}

	cType := COMMIT
	if isMerge {
		cType = MERGE
	}

	cells[pos].setType(cType)
	// if selectionCount > 1 {
	// 	cells[pos].setStyle(sourcePipe.style)
	// }

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
