package gui

import (
	"os"
	"time"

	"github.com/jesseduffield/gocui"
	"github.com/sirupsen/logrus"
)

type SmartRenderListContext struct {
	*BasicContext

	GetItemsLength      func() int
	GetDisplayStrings   func(startIdx int, length int) [][]string
	OnFocus             func() error
	OnFocusLost         func() error
	OnClickSelectedItem func() error

	// the boolean here tells us whether the item is nil. This is needed because you can't work it out on the calling end once the pointer is wrapped in an interface (unless you want to use reflection)
	SelectedItem    func() (ListItem, bool)
	OnGetPanelState func() IListPanelState

	Gui *Gui

	origin int
}

func (self *SmartRenderListContext) GetPanelState() IListPanelState {
	return self.OnGetPanelState()
}

func (self *SmartRenderListContext) GetSelectedItem() (ListItem, bool) {
	return self.SelectedItem()
}

func (self *SmartRenderListContext) GetSelectedItemId() string {
	item, ok := self.GetSelectedItem()

	if !ok {
		return ""
	}

	return item.ID()
}

// OnFocus assumes that the content of the context has already been rendered to the view. OnRender is the function which actually renders the content to the view
func (self *SmartRenderListContext) OnRender() error {
	view, err := self.Gui.g.View(self.ViewName)
	if err != nil {
		return nil
	}

	if self.GetDisplayStrings != nil {
		self.Gui.refreshSelectedLine(self.GetPanelState(), self.GetItemsLength())
		self.reRenderItems(view)
	}

	return nil
}

func (self *SmartRenderListContext) HandleFocusLost() error {
	if self.OnFocusLost != nil {
		return self.OnFocusLost()
	}

	return nil
}

func (self *SmartRenderListContext) FocusLine() {
	view, err := self.Gui.g.View(self.ViewName)
	if err != nil {
		// ignoring error for now
		return
	}

	self.adjustCursorAndOrigin(view, self.GetPanelState().GetSelectedLineIdx())
	view.Footer = formatListFooter(self.GetPanelState().GetSelectedLineIdx(), self.GetItemsLength())
}

func (self *SmartRenderListContext) HandleFocus() error {
	if self.Gui.popupPanelFocused() {
		return nil
	}

	if self.Gui.State.Modes.Diffing.Active() {
		return self.Gui.renderDiff()
	}

	if self.OnFocus != nil {
		return self.OnFocus()
	}

	return nil
}

func (self *SmartRenderListContext) HandleRender() error {
	return self.OnRender()
}

func (self *SmartRenderListContext) handlePrevLine() error {
	return self.handleLineChange(-1)
}

func (self *SmartRenderListContext) handleNextLine() error {
	return self.handleLineChange(1)
}

func (self *SmartRenderListContext) handleLineChange(change int) error {
	if !self.Gui.isPopupPanel(self.ViewName) && self.Gui.popupPanelFocused() {
		return nil
	}

	selectedLineIdx := self.GetPanelState().GetSelectedLineIdx()
	if (change < 0 && selectedLineIdx == 0) || (change > 0 && selectedLineIdx == self.GetItemsLength()-1) {
		return nil
	}

	self.Gui.changeSelectedLine(self.GetPanelState(), self.GetItemsLength(), change)

	return self.HandleFocus()
}

func (self *SmartRenderListContext) adjustCursorAndOrigin(view *gocui.View, selectedLineIdx int) {
	if selectedLineIdx-self.origin < 0 {
		self.origin = selectedLineIdx
		_ = view.SetCursor(0, 0)
	} else if selectedLineIdx-self.origin > view.InnerHeight() {
		self.origin = selectedLineIdx - view.InnerHeight()
		_ = view.SetCursor(0, view.InnerHeight())
	} else {
		_ = view.SetCursor(0, selectedLineIdx-self.origin)
	}

	t := time.Now()
	Log.Warn("about to rerender contents")
	self.reRenderItems(view)
	Log.Warn(time.Since(t))
}

func (self *SmartRenderListContext) reRenderItems(view *gocui.View) {
	// need to get the displaystrings and render them to the view
	displayStrings := self.GetDisplayStrings(self.origin, view.InnerHeight())
	self.Gui.renderDisplayStrings(view, displayStrings)
}

func (self *SmartRenderListContext) handleNextPage() error {
	view, err := self.Gui.g.View(self.ViewName)
	if err != nil {
		return nil
	}
	delta := self.Gui.pageDelta(view)

	return self.handleLineChange(delta)
}

func (self *SmartRenderListContext) handleGotoTop() error {
	return self.handleLineChange(-self.GetItemsLength())
}

func (self *SmartRenderListContext) handleGotoBottom() error {
	return self.handleLineChange(self.GetItemsLength())
}

func (self *SmartRenderListContext) handlePrevPage() error {
	view, err := self.Gui.g.View(self.ViewName)
	if err != nil {
		return nil
	}

	delta := self.Gui.pageDelta(view)

	return self.handleLineChange(-delta)
}

func (self *SmartRenderListContext) handleClick() error {
	if !self.Gui.isPopupPanel(self.ViewName) && self.Gui.popupPanelFocused() {
		return nil
	}

	view, err := self.Gui.g.View(self.ViewName)
	if err != nil {
		return nil
	}

	prevSelectedLineIdx := self.GetPanelState().GetSelectedLineIdx()
	newSelectedLineIdx := view.SelectedLineIdx()

	// we need to focus the view
	if err := self.Gui.pushContext(self); err != nil {
		return err
	}

	if newSelectedLineIdx > self.GetItemsLength()-1 {
		return nil
	}

	self.GetPanelState().SetSelectedLineIdx(newSelectedLineIdx)

	prevViewName := self.Gui.currentViewName()
	if prevSelectedLineIdx == newSelectedLineIdx && prevViewName == self.ViewName && self.OnClickSelectedItem != nil {
		return self.OnClickSelectedItem()
	}
	return self.HandleFocus()
}

func (self *SmartRenderListContext) onSearchSelect(selectedLineIdx int) error {
	self.GetPanelState().SetSelectedLineIdx(selectedLineIdx)
	return self.HandleFocus()
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
