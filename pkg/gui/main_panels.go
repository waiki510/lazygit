package gui

import (
	"os/exec"

	"github.com/jesseduffield/gocui"
)

type ViewUpdateOpts struct {
	Title string

	// awkwardly calling this NoWrap because of how hard Go makes it to have
	// a boolean option that defaults to true
	NoWrap bool

	Highlight bool

	Task updateTask
}

type RefreshMainOpts struct {
	Main      *ViewUpdateOpts
	Secondary *ViewUpdateOpts
}

// constants for updateTask's kind field
type TaskKind int

const (
	RENDER_STRING TaskKind = iota
	RENDER_STRING_WITHOUT_SCROLL
	RUN_FUNCTION
	RUN_COMMAND
	RUN_PTY
)

type updateTask interface {
	GetKind() TaskKind
}

type renderStringTask struct {
	str string
}

func (t *renderStringTask) GetKind() TaskKind {
	return RENDER_STRING
}

func NewRenderStringTask(str string) *renderStringTask {
	return &renderStringTask{str: str}
}

type renderStringWithoutScrollTask struct {
	str string
}

func (t *renderStringWithoutScrollTask) GetKind() TaskKind {
	return RENDER_STRING_WITHOUT_SCROLL
}

func NewRenderStringWithoutScrollTask(str string) *renderStringWithoutScrollTask {
	return &renderStringWithoutScrollTask{str: str}
}

type runCommandTask struct {
	cmd    *exec.Cmd
	prefix string
}

func (t *runCommandTask) GetKind() TaskKind {
	return RUN_COMMAND
}

func NewRunCommandTask(cmd *exec.Cmd) *runCommandTask {
	return &runCommandTask{cmd: cmd}
}

func NewRunCommandTaskWithPrefix(cmd *exec.Cmd, prefix string) *runCommandTask {
	return &runCommandTask{cmd: cmd, prefix: prefix}
}

type runPtyTask struct {
	cmd    *exec.Cmd
	prefix string
}

func (t *runPtyTask) GetKind() TaskKind {
	return RUN_PTY
}

func NewRunPtyTask(cmd *exec.Cmd) *runPtyTask {
	return &runPtyTask{cmd: cmd}
}

// currently unused
// func (gui *Gui) createRunPtyTaskWithPrefix(cmd *exec.Cmd, prefix string) *runPtyTask {
// 	return &runPtyTask{cmd: cmd, prefix: prefix}
// }

type runFunctionTask struct {
	f func(chan struct{}) error
}

func (t *runFunctionTask) GetKind() TaskKind {
	return RUN_FUNCTION
}

// currently unused
// func (gui *Gui) createRunFunctionTask(f func(chan struct{}) error) *runFunctionTask {
// 	return &runFunctionTask{f: f}
// }

func (gui *Gui) runTaskForView(view *gocui.View, task updateTask) error {
	switch task.GetKind() {
	case RENDER_STRING:
		specificTask := task.(*renderStringTask)
		return gui.newStringTask(view, specificTask.str)

	case RENDER_STRING_WITHOUT_SCROLL:
		specificTask := task.(*renderStringWithoutScrollTask)
		return gui.newStringTaskWithoutScroll(view, specificTask.str)

	case RUN_FUNCTION:
		specificTask := task.(*runFunctionTask)
		return gui.newTask(view, specificTask.f)

	case RUN_COMMAND:
		specificTask := task.(*runCommandTask)
		return gui.newCmdTask(view, specificTask.cmd, specificTask.prefix)

	case RUN_PTY:
		specificTask := task.(*runPtyTask)
		return gui.newPtyTask(view, specificTask.cmd, specificTask.prefix)
	}

	return nil
}

func (gui *Gui) refreshMainView(opts *ViewUpdateOpts, view *gocui.View) error {
	view.Title = opts.Title
	view.Wrap = !opts.NoWrap
	view.Highlight = opts.Highlight

	if err := gui.runTaskForView(view, opts.Task); err != nil {
		gui.Log.Error(err)
		return nil
	}

	return nil
}

func (gui *Gui) RefreshMainViews(opts RefreshMainOpts) error {
	if opts.Main != nil {
		if err := gui.refreshMainView(opts.Main, gui.Views.Main); err != nil {
			return err
		}
	}

	if opts.Secondary != nil {
		if err := gui.refreshMainView(opts.Secondary, gui.Views.Secondary); err != nil {
			return err
		}
	}

	gui.splitMainPanel(opts.Secondary != nil)

	return nil
}

func (gui *Gui) splitMainPanel(splitMainPanel bool) {
	gui.State.SplitMainPanel = splitMainPanel
}

func (gui *Gui) isMainPanelSplit() bool {
	return gui.State.SplitMainPanel
}
