package gui

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// list panel functions

func (gui *Gui) getSelectedStashEntry() *models.StashEntry {
	selectedLine := gui.State.Panels.Stash.SelectedLineIdx
	if selectedLine == -1 {
		return nil
	}

	return gui.State.StashEntries[selectedLine]
}

func (gui *Gui) handleStashEntrySelect() error {
	var task updateTask
	stashEntry := gui.getSelectedStashEntry()
	if stashEntry == nil {
		task = NewRenderStringTask(gui.Tr.NoStashEntries)
	} else {
		cmd := gui.OSCommand.ExecutableFromString(
			gui.GitCommand.ShowStashEntryCmdStr(stashEntry.Index),
		)
		task = NewRunPtyTask(cmd)
	}

	return gui.RefreshMainViews(RefreshMainOpts{
		Main: &ViewUpdateOpts{
			Title: "Stash",
			Task:  task,
		},
	})
}

func (gui *Gui) refreshStashEntries() error {
	gui.State.StashEntries = gui.GitCommand.GetStashEntries(gui.State.Modes.Filtering.GetPath())

	return gui.State.Contexts.Stash.HandleRender()
}

// specific functions

func (gui *Gui) handleStashApply() error {
	skipStashWarning := gui.Config.GetUserConfig().Gui.SkipStashWarning

	apply := func() error {
		return gui.stashDo("apply")
	}

	if skipStashWarning {
		return apply()
	}

	return gui.Ask(AskOpts{
		Title:  gui.Tr.StashApply,
		Prompt: gui.Tr.SureApplyStashEntry,
		HandleConfirm: func() error {
			return apply()
		},
	})
}

func (gui *Gui) handleStashPop() error {
	skipStashWarning := gui.Config.GetUserConfig().Gui.SkipStashWarning

	pop := func() error {
		return gui.stashDo("pop")
	}

	if skipStashWarning {
		return pop()
	}

	return gui.Ask(AskOpts{
		Title:  gui.Tr.StashPop,
		Prompt: gui.Tr.SurePopStashEntry,
		HandleConfirm: func() error {
			return pop()
		},
	})
}

func (gui *Gui) handleStashDrop() error {
	return gui.Ask(AskOpts{
		Title:  gui.Tr.StashDrop,
		Prompt: gui.Tr.SureDropStashEntry,
		HandleConfirm: func() error {
			return gui.stashDo("drop")
		},
	})
}

func (gui *Gui) stashDo(method string) error {
	stashEntry := gui.getSelectedStashEntry()
	if stashEntry == nil {
		errorMessage := utils.ResolvePlaceholderString(
			gui.Tr.NoStashTo,
			map[string]string{
				"method": method,
			},
		)

		return gui.CreateErrorPanel(errorMessage)
	}
	if err := gui.GitCommand.WithSpan(gui.Tr.Spans.Stash).StashDo(stashEntry.Index, method); err != nil {
		return gui.SurfaceError(err)
	}
	return gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{STASH, FILES}})
}

func (gui *Gui) handleViewStashFiles() error {
	stashEntry := gui.getSelectedStashEntry()
	if stashEntry == nil {
		return nil
	}

	return gui.switchToCommitFilesContext(stashEntry.RefName(), false, gui.State.Contexts.Stash, "stash")
}
