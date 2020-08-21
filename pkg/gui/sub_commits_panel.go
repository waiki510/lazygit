package gui

import (
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands"
)

// list panel functions

func (gui *Gui) getSelectedSubCommit() *commands.Commit {
	selectedLine := gui.State.Panels.SubCommits.SelectedLineIdx
	commits := gui.State.SubCommits
	if selectedLine == -1 || len(commits) == 0 {
		return nil
	}

	return commits[selectedLine]
}

func (gui *Gui) handleSubCommitSelect() error {
	commit := gui.getSelectedSubCommit()
	var task updateTask
	if commit == nil {
		task = gui.createRenderStringTask("No commits")
	} else {
		cmd := gui.OSCommand.ExecutableFromString(
			gui.GitCommand.ShowCmdStr(commit.Sha, gui.State.FilterPath),
		)

		task = gui.createRunPtyTask(cmd)
	}

	return gui.refreshMain(refreshMainOpts{
		main: &viewUpdateOpts{
			title: "Commit",
			task:  task,
		},
	})
}

func (gui *Gui) handleCheckoutSubCommit(g *gocui.Gui, v *gocui.View) error {
	commit := gui.getSelectedSubCommit()
	if commit == nil {
		return nil
	}

	err := gui.ask(askOpts{
		returnToView:       gui.getCommitsView(),
		returnFocusOnClose: true,
		title:              gui.Tr.SLocalize("checkoutCommit"),
		prompt:             gui.Tr.SLocalize("SureCheckoutThisCommit"),
		handleConfirm: func() error {
			return gui.handleCheckoutRef(commit.Sha, handleCheckoutRefOptions{})
		},
	})
	if err != nil {
		return err
	}

	gui.State.Panels.SubCommits.SelectedLineIdx = 0

	return nil
}

func (gui *Gui) handleCreateSubCommitResetMenu() error {
	commit := gui.getSelectedSubCommit()

	return gui.createResetMenu(commit.Sha)
}

func (gui *Gui) handleViewSubCommitFiles() error {
	commit := gui.getSelectedSubCommit()
	if commit == nil {
		return nil
	}

	return gui.switchToCommitFilesContext(commit.Sha, REF_TYPE_OTHER_COMMIT, gui.Contexts.SubCommits.Context, "branches")
}
