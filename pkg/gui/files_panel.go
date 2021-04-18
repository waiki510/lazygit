package gui

import (
	"fmt"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

// list panel functions

func (gui *Gui) GetSelectedFileNode() *filetree.FileNode {
	selectedLine := gui.State.Panels.Files.SelectedLineIdx
	if selectedLine == -1 {
		return nil
	}

	return gui.State.FileManager.GetItemAtIndex(selectedLine)
}

func (gui *Gui) GetSelectedFile() *models.File {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}
	return node.File
}

func (gui *Gui) GetSelectedPath() string {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return ""
	}

	return node.GetPath()
}

func (gui *Gui) SelectFile(alreadySelected bool) error {
	gui.Views.Files.FocusPoint(0, gui.State.Panels.Files.SelectedLineIdx)

	node := gui.GetSelectedFileNode()

	if node == nil {
		return gui.RefreshMainViews(RefreshMainOpts{
			Main: &ViewUpdateOpts{
				Title: "",
				Task:  NewRenderStringTask(gui.Tr.NoChangedFiles),
			},
		})
	}

	if !alreadySelected {
		// TODO: pull into update task interface
		if err := gui.resetOrigin(gui.Views.Main); err != nil {
			return err
		}
		if err := gui.resetOrigin(gui.Views.Secondary); err != nil {
			return err
		}
		gui.takeOverMergeConflictScrolling()
	}

	if node.File != nil && node.File.HasInlineMergeConflicts {
		return gui.refreshMergePanelWithLock()
	}

	cmdStr := gui.GitCommand.WorktreeFileDiffCmdStr(node, false, !node.GetHasUnstagedChanges() && node.GetHasStagedChanges())
	cmd := gui.OSCommand.ExecutableFromString(cmdStr)

	refreshOpts := RefreshMainOpts{Main: &ViewUpdateOpts{
		Title: gui.Tr.UnstagedChanges,
		Task:  NewRunPtyTask(cmd),
	}}

	if node.GetHasUnstagedChanges() {
		if node.GetHasStagedChanges() {
			cmdStr := gui.GitCommand.WorktreeFileDiffCmdStr(node, false, true)
			cmd := gui.OSCommand.ExecutableFromString(cmdStr)

			refreshOpts.Secondary = &ViewUpdateOpts{
				Title: gui.Tr.StagedChanges,
				Task:  NewRunPtyTask(cmd),
			}
		}
	} else {
		refreshOpts.Main.Title = gui.Tr.StagedChanges
	}

	return gui.RefreshMainViews(refreshOpts)
}

func (gui *Gui) RefreshFilesAndSubmodules() error {
	gui.Mutexes.RefreshingFilesMutex.Lock()
	gui.State.IsRefreshingFiles = true
	defer func() {
		gui.State.IsRefreshingFiles = false
		gui.Mutexes.RefreshingFilesMutex.Unlock()
	}()

	selectedPath := gui.GetSelectedPath()

	if err := gui.refreshStateSubmoduleConfigs(); err != nil {
		return err
	}
	if err := gui.refreshStateFiles(); err != nil {
		return err
	}

	gui.g.Update(func(g *gocui.Gui) error {
		if err := gui.PostRefreshUpdate(gui.State.Contexts.Submodules); err != nil {
			gui.Log.Error(err)
		}

		if ContextKey(gui.Views.Files.Context) == FILES_CONTEXT_KEY {
			// doing this a little custom (as opposed to using gui.postRefreshUpdate) because we handle selecting the file explicitly below
			if err := gui.State.Contexts.Files.HandleRender(); err != nil {
				return err
			}
		}

		if gui.currentContext().GetKey() == FILES_CONTEXT_KEY || (g.CurrentView() == gui.Views.Main && ContextKey(g.CurrentView().Context) == MAIN_MERGING_CONTEXT_KEY) {
			newSelectedPath := gui.GetSelectedPath()
			alreadySelected := selectedPath != "" && newSelectedPath == selectedPath
			if err := gui.SelectFile(alreadySelected); err != nil {
				return err
			}
		}

		return nil
	})

	return nil
}

// specific functions

func (gui *Gui) StagedFiles() []*models.File {
	files := gui.State.FileManager.GetAllFiles()
	result := make([]*models.File, 0)
	for _, file := range files {
		if file.HasStagedChanges {
			result = append(result, file)
		}
	}
	return result
}

func (gui *Gui) TrackedFiles() []*models.File {
	files := gui.State.FileManager.GetAllFiles()
	result := make([]*models.File, 0, len(files))
	for _, file := range files {
		if file.Tracked {
			result = append(result, file)
		}
	}
	return result
}

func (gui *Gui) EnterFile(forceSecondaryFocused bool, selectedLineIdx int) error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	if node.File == nil {
		return gui.toggleDirCollapsed()
	}

	file := node.File

	submoduleConfigs := gui.State.Submodules
	if file.IsSubmodule(submoduleConfigs) {
		submoduleConfig := file.SubmoduleConfig(submoduleConfigs)
		return gui.enterSubmodule(submoduleConfig)
	}

	if file.HasInlineMergeConflicts {
		return gui.SwitchToMerge()
	}
	if file.HasMergeConflicts {
		return gui.CreateErrorPanel(gui.Tr.FileStagingRequirements)
	}
	_ = gui.PushContext(gui.State.Contexts.Staging)

	return gui.handleRefreshStagingPanel(forceSecondaryFocused, selectedLineIdx) // TODO: check if this is broken, try moving into context code
}

func (gui *Gui) toggleDirCollapsed() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	gui.State.FileManager.ToggleCollapsed(node.GetPath())

	if err := gui.PostRefreshUpdate(gui.State.Contexts.Files); err != nil {
		gui.Log.Error(err)
	}

	return nil
}

func (gui *Gui) focusAndSelectFile() error {
	return gui.SelectFile(false)
}

func (gui *Gui) EditFile(filename string) error {
	cmdStr, err := gui.GitCommand.EditFileCmdStr(filename)
	if err != nil {
		return gui.SurfaceError(err)
	}

	return gui.RunSubprocessWithSuspenseAndRefresh(
		gui.OSCommand.WithSpan(gui.Tr.Spans.EditFile).PrepareShellSubProcess(cmdStr),
	)
}

func (gui *Gui) refreshStateFiles() error {
	state := gui.State

	// keep track of where the cursor is currently and the current file names
	// when we refresh, go looking for a matching name
	// move the cursor to there.

	selectedNode := gui.GetSelectedFileNode()

	prevNodes := gui.State.FileManager.GetAllItems()
	prevSelectedLineIdx := gui.State.Panels.Files.SelectedLineIdx

	files := gui.GitCommand.GetStatusFiles(commands.GetStatusFileOptions{})

	// for when you stage the old file of a rename and the new file is in a collapsed dir
	state.FileManager.RWMutex.Lock()
	for _, file := range files {
		if selectedNode != nil && selectedNode.Path != "" && file.PreviousName == selectedNode.Path {
			state.FileManager.ExpandToPath(file.Name)
		}
	}

	state.FileManager.SetFiles(files)
	state.FileManager.RWMutex.Unlock()

	if err := gui.fileWatcher.addFilesToFileWatcher(files); err != nil {
		return err
	}

	if selectedNode != nil {
		newIdx := gui.findNewSelectedIdx(prevNodes[prevSelectedLineIdx:], state.FileManager.GetAllItems())
		if newIdx != -1 && newIdx != prevSelectedLineIdx {
			newNode := state.FileManager.GetItemAtIndex(newIdx)
			// when not in tree mode, we show merge conflict files at the top, so you
			// can work through them one by one without having to sift through a large
			// set of files. If you have just fixed the merge conflicts of a file, we
			// actually don't want to jump to that file's new position, because that
			// file will now be ages away amidst the other files without merge
			// conflicts: the user in this case would rather work on the next file
			// with merge conflicts, which will have moved up to fill the gap left by
			// the last file, meaning the cursor doesn't need to move at all.
			leaveCursor := !state.FileManager.InTreeMode() && newNode != nil &&
				selectedNode.File != nil && selectedNode.File.HasMergeConflicts &&
				newNode.File != nil && !newNode.File.HasMergeConflicts

			if !leaveCursor {
				state.Panels.Files.SelectedLineIdx = newIdx
			}
		}
	}

	gui.refreshSelectedLine(state.Panels.Files, state.FileManager.GetItemsLength())
	return nil
}

// Let's try to find our file again and move the cursor to that.
// If we can't find our file, it was probably just removed by the user. In that
// case, we go looking for where the next file has been moved to. Given that the
// user could have removed a whole directory, we continue iterating through the old
// nodes until we find one that exists in the new set of nodes, then move the cursor
// to that.
// prevNodes starts from our previously selected node because we don't need to consider anything above that
func (gui *Gui) findNewSelectedIdx(prevNodes []*filetree.FileNode, currNodes []*filetree.FileNode) int {
	getPaths := func(node *filetree.FileNode) []string {
		if node == nil {
			return nil
		}
		if node.File != nil && node.File.IsRename() {
			return node.File.Names()
		} else {
			return []string{node.Path}
		}
	}

	for _, prevNode := range prevNodes {
		selectedPaths := getPaths(prevNode)

		for idx, node := range currNodes {
			paths := getPaths(node)

			// If you started off with a rename selected, and now it's broken in two, we want you to jump to the new file, not the old file.
			// This is because the new should be in the same position as the rename was meaning less cursor jumping
			foundOldFileInRename := prevNode.File != nil && prevNode.File.IsRename() && node.Path == prevNode.File.PreviousName
			foundNode := utils.StringArraysOverlap(paths, selectedPaths) && !foundOldFileInRename
			if foundNode {
				return idx
			}
		}
	}

	return -1
}

type PullFilesOptions struct {
	RemoteName string
	BranchName string
	span       string
}

func (gui *Gui) PullWithMode(mode string, opts PullFilesOptions) error {
	gui.Mutexes.FetchMutex.Lock()
	defer gui.Mutexes.FetchMutex.Unlock()

	gitCommand := gui.GitCommand.WithSpan(opts.span)

	err := gitCommand.Fetch(
		commands.FetchOptions{
			PromptUserForCredential: gui.PromptUserForCredential,
			RemoteName:              opts.RemoteName,
			BranchName:              opts.BranchName,
		},
	)
	gui.HandleCredentialsPopup(err)
	if err != nil {
		return gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC})
	}

	switch mode {
	case "rebase":
		err := gitCommand.RebaseBranch("FETCH_HEAD")
		return gui.handleGenericMergeCommandResult(err)
	case "merge":
		err := gitCommand.Merge("FETCH_HEAD", commands.MergeOpts{})
		return gui.handleGenericMergeCommandResult(err)
	case "ff-only":
		err := gitCommand.Merge("FETCH_HEAD", commands.MergeOpts{FastForwardOnly: true})
		return gui.handleGenericMergeCommandResult(err)
	default:
		return gui.CreateErrorPanel(fmt.Sprintf("git pull mode '%s' unrecognised", mode))
	}
}

func (gui *Gui) SwitchToMerge() error {
	file := gui.GetSelectedFile()
	if file == nil {
		return nil
	}

	if !file.HasInlineMergeConflicts {
		return gui.CreateErrorPanel(gui.Tr.FileNoMergeCons)
	}

	return gui.PushContext(gui.State.Contexts.Merging)
}

func (gui *Gui) OpenFile(filename string) error {
	if err := gui.OSCommand.WithSpan(gui.Tr.Spans.OpenFile).OpenFile(filename); err != nil {
		return gui.SurfaceError(err)
	}
	return nil
}
