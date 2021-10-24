package presentation

import (
	"os"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sirupsen/logrus"
)

func renderCommitGraph(commits []*models.Commit, selectedCommit *models.Commit) []string {
	if len(commits) == 0 {
		return nil
	}

	// unlikely to have merges 10 levels deep
	paths := make([]Path, 0, 10)
	paths = append(paths, Path{from: "START", to: commits[0].Sha, prevPos: 0})

	output := make([]string, 0, len(commits))
	for _, commit := range commits {
		var line string
		line, paths = renderLine(commit, paths, selectedCommit)
		output = append(output, line)
	}

	return output
}

type Path struct {
	from    string
	to      string
	prevPos int
}

func equalHashes(a, b string) bool {
	length := utils.Min(len(a), len(b))
	// parent hashes are only stored up to 20 characters for some reason so we'll truncate to that for comparison
	return a[:length] == b[:length]
}

type cellType int

const (
	CONNECTION cellType = iota
	COMMIT
	MERGE
)

type Cell struct {
	up, down, left, right                                     bool
	highlightUp, highlightDown, highlightLeft, highlightRight bool
	cellType                                                  cellType
	highlight                                                 bool
}

func (cell *Cell) render() string {
	str := cell.renderString()
	if cell.highlight || cell.highlightUp || cell.highlightDown || cell.highlightLeft || cell.highlightRight {
		str = style.FgMagenta.Sprint(str)
	}
	return str
}

func (cell *Cell) renderString() string {
	up, down, left, right := cell.up, cell.down, cell.left, cell.right
	if cell.highlightUp || cell.highlightDown || cell.highlightLeft || cell.highlightRight {
		up, down, left, right = cell.highlightUp, cell.highlightDown, cell.highlightLeft, cell.highlightRight
	}
	first, second := getBoxDrawingChars(up, down, left, right)
	switch cell.cellType {
	case CONNECTION:
		return string(first) + string(second)
	case COMMIT:
		return "o" + string(second)
	case MERGE:
		return "M" + string(second)
	}

	panic("unreachable")
}

func (cell *Cell) setUp(highlight bool) *Cell {
	cell.up = true
	if highlight {
		cell.highlightUp = true
	}
	return cell
}

func (cell *Cell) setDown(highlight bool) *Cell {
	cell.down = true
	if highlight {
		cell.highlightDown = true
	}
	return cell
}

func (cell *Cell) setLeft(highlight bool) *Cell {
	cell.left = true
	if highlight {
		cell.highlightLeft = true
	}
	return cell
}

func (cell *Cell) setRight(highlight bool) *Cell {
	cell.right = true
	if highlight {
		cell.highlightRight = true
	}
	return cell
}

func (cell *Cell) setHighlight() *Cell {
	cell.highlight = true
	return cell
}

func (cell *Cell) setType(cellType cellType) *Cell {
	cell.cellType = cellType
	return cell
}

func getMaxPrevPosition(paths []Path) int {
	max := 0
	for _, path := range paths {
		if path.prevPos > max {
			max = path.prevPos
		}
	}
	return max
}

func renderLine(commit *models.Commit, paths []Path, selectedCommit *models.Commit) (string, []Path) {
	line := ""

	pos := -1
	terminating := 0
	for i, path := range paths {
		if equalHashes(path.to, commit.Sha) {
			if pos == -1 {
				pos = i
			}
			terminating++
		}
	}
	if pos == -1 {
		Log.Warnf("no parent for commit %s", commit.Sha)
	}

	// find the first position available if you're a merge commit
	newPathPos := -1
	if commit.IsMerge() {
		newPathPos = len(paths) - terminating + 1
	}

	cellLength := len(paths)
	if newPathPos > cellLength-1 {
		cellLength = newPathPos + 1
	}
	maxPrevPos := getMaxPrevPosition(paths)
	if maxPrevPos > cellLength-1 {
		cellLength = maxPrevPos + 1
	}

	isSelected := equalHashes(commit.Sha, selectedCommit.Sha)

	cells := make([]*Cell, cellLength)
	for i := 0; i < cellLength; i++ {
		cells[i] = &Cell{}
	}
	if isSelected {
		cells[pos].setHighlight()
	}
	if commit.IsMerge() {
		cells[pos].setType(MERGE).setRight(isSelected)
		cells[newPathPos].setLeft(isSelected).setDown(isSelected)
		for i := pos + 1; i < newPathPos; i++ {
			cells[i].setLeft(isSelected).setRight(isSelected)
		}
	} else {
		cells[pos].setType(COMMIT)
	}

	connectHorizontal := func(x1, x2 int, highlight bool) {
		cells[x1].setRight(highlight)
		cells[x2].setLeft(highlight)
		for i := x1 + 1; i < x2; i++ {
			cells[i].setLeft(highlight).setRight(highlight)
		}
	}

	for i, path := range paths {
		// get path from previous to current position
		highlightPath := equalHashes(path.from, selectedCommit.Sha)
		cells[path.prevPos].setUp(highlightPath)
		if path.prevPos != i {
			connectHorizontal(i, path.prevPos, highlightPath)
		}

		if equalHashes(path.to, commit.Sha) {
			if i == pos {
				continue
			}
			connectHorizontal(pos, i, highlightPath)
		} else {
			// check this
			cells[i].setDown(highlightPath)
		}
	}

	for _, cell := range cells {
		line += cell.render()
	}

	paths[pos] = Path{from: commit.Sha, to: commit.Parents[0], prevPos: pos}
	newPaths := make([]Path, 0, len(paths)+1)
	for i, path := range paths {
		if !equalHashes(path.to, commit.Sha) {
			path.prevPos = i
			newPaths = append(newPaths, path)
		}
	}
	if commit.IsMerge() {
		newPaths = append(newPaths, Path{from: commit.Sha, to: commit.Parents[1], prevPos: newPathPos})
	}

	return line, newPaths
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

func getBoxDrawingChars(up, down, left, right bool) (rune, rune) {
	if up && down && left && right {
		return '┼', '─'
	} else if up && down && left && !right {
		return '┤', ' '
	} else if up && down && !left && right {
		return '│', '─'
	} else if up && down && !left && !right {
		return '│', ' '
	} else if up && !down && left && right {
		return '┴', '─'
	} else if up && !down && left && !right {
		return '┘', ' '
	} else if up && !down && !left && right {
		return '└', '─'
	} else if up && !down && !left && !right {
		return '└', ' '
	} else if !up && down && left && right {
		return '┬', '─'
	} else if !up && down && left && !right {
		return '┐', ' '
	} else if !up && down && !left && right {
		return '┌', '─'
	} else if !up && down && !left && !right {
		return '╷', ' '
	} else if !up && !down && left && right {
		return '─', '─'
	} else if !up && !down && left && !right {
		return '─', ' '
	} else if !up && !down && !left && right {
		return '╶', '─'
	} else if !up && !down && !left && !right {
		return ' ', ' '
	} else {
		panic("should not be possible")
	}
}
