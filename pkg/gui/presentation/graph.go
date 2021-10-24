package presentation

import (
	"math/rand"
	"os"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/style"
	"github.com/jesseduffield/lazygit/pkg/utils"
	"github.com/sirupsen/logrus"
)

const mergeSymbol = "⏣" //Ɏ
const commitSymbol = "⎔"

// I'll take a start index

type Line struct {
	isHighlighted bool
	content       string
}

func renderCommitGraph(commits []*models.Commit, selectedCommit *models.Commit) ([]Blah, []Line) {
	if len(commits) == 0 {
		return nil, nil
	}

	// unlikely to have merges 10 levels deep
	paths := make([]Path, 0, 10)
	paths = append(paths, Path{from: "START", to: commits[0].Sha, prevPos: 0, style: getRandStyle()})

	lines := make([]Line, 0, len(commits))
	blahs := []Blah{}
	for _, commit := range commits {
		var line Line
		var blah Blah
		line, blah, paths = renderLine(commit, paths, selectedCommit)
		blahs = append(blahs, blah)
		lines = append(lines, line)
	}

	return blahs, lines
}

type Path struct {
	from    string
	to      string
	prevPos int
	style   style.TextStyle
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
	style                                                     style.TextStyle
}

var styles = []style.TextStyle{
	style.FgCyan,
	style.FgBlue,
	style.FgGreen,
	style.FgYellow,
	style.FgMagenta,
	style.FgRed,
}

func (cell *Cell) render() string {
	str := cell.renderString()
	if cell.isHighlighted() {
		str = style.FgMagenta.Sprint(str)
	} else {
		str = cell.style.Sprint(str)
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
		return commitSymbol + string(second)
	case MERGE:
		return mergeSymbol + string(second)
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

func (cell *Cell) setStyle(style style.TextStyle) *Cell {
	cell.style = style
	return cell
}

func (cell *Cell) setType(cellType cellType) *Cell {
	cell.cellType = cellType
	return cell
}

func (cell *Cell) isHighlighted() bool {
	return (cell.highlight || cell.highlightUp || cell.highlightDown || cell.highlightLeft || cell.highlightRight) && !(cell.cellType != CONNECTION && !cell.highlight)
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

func getRandStyle() style.TextStyle {
	// no colours for now
	return style.FgDefault

	return styles[rand.Intn(len(styles))]
}

func createLine(blah Blah, selectedCommit *models.Commit) Line {
	paths := blah.paths
	commit := blah.commit
	pos := blah.pos
	newPathPos := blah.newPathPos

	cellLength := len(paths)
	if newPathPos > cellLength-1 {
		cellLength = newPathPos + 1
	}
	maxPrevPos := getMaxPrevPosition(paths)
	if maxPrevPos > cellLength-1 {
		cellLength = maxPrevPos + 1
	}

	isSelected := equalHashes(commit.Sha, selectedCommit.Sha)
	isParentOfSelected := false
	for _, parentSha := range selectedCommit.Parents {
		if equalHashes(parentSha, commit.Sha) {
			isParentOfSelected = true
			break
		}
	}

	cells := make([]*Cell, cellLength)
	for i := 0; i < cellLength; i++ {
		cells[i] = &Cell{style: style.FgWhite}
	}
	if isSelected || isParentOfSelected {
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

	connectHorizontal := func(x1, x2 int, highlight bool, style style.TextStyle) {
		cells[x1].setRight(highlight).setStyle(style)
		cells[x2].setLeft(highlight).setStyle(style)
		for i := x1 + 1; i < x2; i++ {
			cells[i].setLeft(highlight).setRight(highlight).setStyle(style)
		}
	}

	for i, path := range paths {
		// get path from previous to current position
		highlightPath := equalHashes(path.from, selectedCommit.Sha)
		cells[path.prevPos].setUp(highlightPath)
		if path.prevPos != i {
			connectHorizontal(i, path.prevPos, highlightPath, path.style)
		}

		if equalHashes(path.to, commit.Sha) {
			if i == pos {
				continue
			}
			connectHorizontal(pos, i, highlightPath, path.style)
		} else {
			// check this
			cells[i].setDown(highlightPath).setStyle(path.style)
		}
	}

	line := Line{content: "", isHighlighted: false}
	for _, cell := range cells {
		line.content += cell.render()
		if cell.isHighlighted() {
			line.isHighlighted = true
		}
	}

	return line
}

type Blah struct {
	commit     *models.Commit
	paths      []Path
	pos        int
	newPathPos int
}

func renderLine(commit *models.Commit, paths []Path, selectedCommit *models.Commit) (Line, Blah, []Path) {
	pos := -1
	terminating := 0
	for i, path := range paths {
		if equalHashes(path.to, commit.Sha) {
			// if we haven't seen this before, what do we do? Treat it like it's got random parents?
			if pos == -1 {
				pos = i
			}
			terminating++
		}
	}
	if pos == -1 {
		// this can happen when doing `git log --all`
		pos = len(paths)
		// pick a random style
		paths = append(paths, Path{from: "START", to: commit.Sha, prevPos: pos, style: getRandStyle()})
	}

	// find the first position available if you're a merge commit
	newPathPos := -1
	if commit.IsMerge() {
		newPathPos = len(paths) - terminating + 1
	}

	blah := Blah{commit: commit, paths: paths, pos: pos, newPathPos: newPathPos}
	line := createLine(blah, selectedCommit)

	paths[pos] = Path{from: commit.Sha, to: commit.Parents[0], prevPos: pos, style: getRandStyle()}
	newPaths := make([]Path, 0, len(paths)+1)
	for i, path := range paths {
		if !equalHashes(path.to, commit.Sha) {
			path.prevPos = i
			newPaths = append(newPaths, path)
		}
	}
	if commit.IsMerge() {
		newPaths = append(newPaths, Path{from: commit.Sha, to: commit.Parents[1], prevPos: newPathPos, style: getRandStyle()})
	}

	return line, blah, newPaths
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
