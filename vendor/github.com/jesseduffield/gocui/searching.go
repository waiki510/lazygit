package gocui

type searcher struct {
	searchString       string
	searchPositions    []cellPos
	currentSearchIndex int
	onSelectItem       func(int, int, int) error
}

func (s *searcher) search(str string) {
	s.searchString = str
	s.searchPositions = []cellPos{}
	s.currentSearchIndex = 0
}

func (s *searcher) clearSearch() {
	s.searchString = ""
	s.searchPositions = []cellPos{}
	s.currentSearchIndex = 0
}

func (v *View) SetOnSelectItem(onSelectItem func(int, int, int) error) {
	v.searcher.onSelectItem = onSelectItem
}

func (v *View) gotoNextMatch() error {
	if len(v.searcher.searchPositions) == 0 {
		return nil
	}
	if v.searcher.currentSearchIndex >= len(v.searcher.searchPositions)-1 {
		v.searcher.currentSearchIndex = 0
	} else {
		v.searcher.currentSearchIndex++
	}
	return v.SelectSearchResult(v.searcher.currentSearchIndex)
}

func (v *View) gotoPreviousMatch() error {
	if len(v.searcher.searchPositions) == 0 {
		return nil
	}
	if v.searcher.currentSearchIndex == 0 {
		if len(v.searcher.searchPositions) > 0 {
			v.searcher.currentSearchIndex = len(v.searcher.searchPositions) - 1
		}
	} else {
		v.searcher.currentSearchIndex--
	}
	return v.SelectSearchResult(v.searcher.currentSearchIndex)
}

func (v *View) SelectSearchResult(index int) error {
	itemCount := len(v.searcher.searchPositions)
	if itemCount == 0 {
		return nil
	}
	if index > itemCount-1 {
		index = itemCount - 1
	}

	y := v.searcher.searchPositions[index].y
	v.FocusPoint(0, y)
	if v.searcher.onSelectItem != nil {
		return v.searcher.onSelectItem(y, index, itemCount)
	}
	return nil
}

func (v *View) Search(str string) error {
	v.writeMutex.Lock()
	defer v.writeMutex.Unlock()

	v.searcher.search(str)
	v.updateSearchPositions()
	if len(v.searcher.searchPositions) > 0 {
		// get the first result past the current cursor
		currentIndex := 0
		adjustedY := v.oy + v.cy
		adjustedX := v.ox + v.cx
		for i, pos := range v.searcher.searchPositions {
			if pos.y > adjustedY || (pos.y == adjustedY && pos.x > adjustedX) {
				currentIndex = i
				break
			}
		}
		v.searcher.currentSearchIndex = currentIndex
		return v.SelectSearchResult(currentIndex)
	} else {
		return v.searcher.onSelectItem(-1, -1, 0)
	}
}

func (v *View) ClearSearch() {
	v.searcher.clearSearch()
}

func (v *View) IsSearching() bool {
	return v.searcher.searchString != ""
}
