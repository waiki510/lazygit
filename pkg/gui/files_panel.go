package gui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/config"
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

func (gui *Gui) refreshFilesAndSubmodules() error {
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
		if err := gui.postRefreshUpdate(gui.State.Contexts.Submodules); err != nil {
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

func (gui *Gui) stagedFiles() []*models.File {
	files := gui.State.FileManager.GetAllFiles()
	result := make([]*models.File, 0)
	for _, file := range files {
		if file.HasStagedChanges {
			result = append(result, file)
		}
	}
	return result
}

func (gui *Gui) trackedFiles() []*models.File {
	files := gui.State.FileManager.GetAllFiles()
	result := make([]*models.File, 0, len(files))
	for _, file := range files {
		if file.Tracked {
			result = append(result, file)
		}
	}
	return result
}

func (gui *Gui) HandleEnterFile() error {
	return gui.EnterFile(false, -1)
}

func (gui *Gui) EnterFile(forceSecondaryFocused bool, selectedLineIdx int) error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	if node.File == nil {
		return gui.HandleToggleDirCollapsed()
	}

	file := node.File

	submoduleConfigs := gui.State.Submodules
	if file.IsSubmodule(submoduleConfigs) {
		submoduleConfig := file.SubmoduleConfig(submoduleConfigs)
		return gui.enterSubmodule(submoduleConfig)
	}

	if file.HasInlineMergeConflicts {
		return gui.HandleSwitchToMerge()
	}
	if file.HasMergeConflicts {
		return gui.CreateErrorPanel(gui.Tr.FileStagingRequirements)
	}
	_ = gui.PushContext(gui.State.Contexts.Staging)

	return gui.handleRefreshStagingPanel(forceSecondaryFocused, selectedLineIdx) // TODO: check if this is broken, try moving into context code
}

func (gui *Gui) HandleFilePress() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	if node.IsLeaf() {
		file := node.File

		if file.HasInlineMergeConflicts {
			return gui.HandleSwitchToMerge()
		}

		if file.HasUnstagedChanges {
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.StageFile).StageFile(file.Name); err != nil {
				return gui.SurfaceError(err)
			}
		} else {
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.UnstageFile).UnStageFile(file.Names(), file.Tracked); err != nil {
				return gui.SurfaceError(err)
			}
		}
	} else {
		// if any files within have inline merge conflicts we can't stage or unstage,
		// or it'll end up with those >>>>>> lines actually staged
		if node.GetHasInlineMergeConflicts() {
			return gui.CreateErrorPanel(gui.Tr.ErrStageDirWithInlineMergeConflicts)
		}

		if node.GetHasUnstagedChanges() {
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.StageFile).StageFile(node.Path); err != nil {
				return gui.SurfaceError(err)
			}
		} else {
			// pretty sure it doesn't matter that we're always passing true here
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.UnstageFile).UnStageFile([]string{node.Path}, true); err != nil {
				return gui.SurfaceError(err)
			}
		}
	}

	if err := gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{FILES}}); err != nil {
		return err
	}

	return gui.SelectFile(true)
}

func (gui *Gui) allFilesStaged() bool {
	for _, file := range gui.State.FileManager.GetAllFiles() {
		if file.HasUnstagedChanges {
			return false
		}
	}
	return true
}

func (gui *Gui) focusAndSelectFile() error {
	return gui.SelectFile(false)
}

func (gui *Gui) HandleStageAll() error {
	var err error
	if gui.allFilesStaged() {
		err = gui.GitCommand.WithSpan(gui.Tr.Spans.UnstageAllFiles).UnstageAll()
	} else {
		err = gui.GitCommand.WithSpan(gui.Tr.Spans.StageAllFiles).StageAll()
	}
	if err != nil {
		_ = gui.SurfaceError(err)
	}

	if err := gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{FILES}}); err != nil {
		return err
	}

	return gui.SelectFile(false)
}

func (gui *Gui) HandleIgnoreFile() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	if node.GetPath() == ".gitignore" {
		return gui.CreateErrorPanel("Cannot ignore .gitignore")
	}

	gitCommand := gui.GitCommand.WithSpan(gui.Tr.Spans.IgnoreFile)

	unstageFiles := func() error {
		return node.ForEachFile(func(file *models.File) error {
			if file.HasStagedChanges {
				if err := gitCommand.UnStageFile(file.Names(), file.Tracked); err != nil {
					return err
				}
			}

			return nil
		})
	}

	if node.GetIsTracked() {
		return gui.Ask(AskOpts{
			Title:  gui.Tr.IgnoreTracked,
			Prompt: gui.Tr.IgnoreTrackedPrompt,
			HandleConfirm: func() error {
				// not 100% sure if this is necessary but I'll assume it is
				if err := unstageFiles(); err != nil {
					return err
				}

				if err := gitCommand.RemoveTrackedFiles(node.GetPath()); err != nil {
					return err
				}

				if err := gitCommand.Ignore(node.GetPath()); err != nil {
					return err
				}
				return gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{FILES}})
			},
		})
	}

	if err := unstageFiles(); err != nil {
		return err
	}

	if err := gitCommand.Ignore(node.GetPath()); err != nil {
		return gui.SurfaceError(err)
	}

	return gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{FILES}})
}

func (gui *Gui) HandleWIPCommitPress() error {
	skipHookPrefix := gui.Config.GetUserConfig().Git.SkipHookPrefix
	if skipHookPrefix == "" {
		return gui.CreateErrorPanel(gui.Tr.SkipHookPrefixNotConfigured)
	}

	if err := gui.Views.CommitMessage.SetEditorContent(skipHookPrefix); err != nil {
		return err
	}

	return gui.HandleCommitPress()
}

func (gui *Gui) commitPrefixConfigForRepo() *config.CommitPrefixConfig {
	cfg, ok := gui.Config.GetUserConfig().Git.CommitPrefixes[utils.GetCurrentRepoName()]
	if !ok {
		return nil
	}

	return &cfg
}

func (gui *Gui) prepareFilesForCommit() error {
	noStagedFiles := len(gui.stagedFiles()) == 0
	if noStagedFiles && gui.Config.GetUserConfig().Gui.SkipNoStagedFilesWarning {
		err := gui.GitCommand.WithSpan(gui.Tr.Spans.StageAllFiles).StageAll()
		if err != nil {
			return err
		}

		return gui.refreshFilesAndSubmodules()
	}

	return nil
}

func (gui *Gui) HandleCommitPress() error {
	if err := gui.prepareFilesForCommit(); err != nil {
		return gui.SurfaceError(err)
	}

	if gui.State.FileManager.GetItemsLength() == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoFilesStagedTitle)
	}

	if len(gui.stagedFiles()) == 0 {
		return gui.promptToStageAllAndRetry(gui.HandleCommitPress)
	}

	commitPrefixConfig := gui.commitPrefixConfigForRepo()
	if commitPrefixConfig != nil {
		prefixPattern := commitPrefixConfig.Pattern
		prefixReplace := commitPrefixConfig.Replace
		rgx, err := regexp.Compile(prefixPattern)
		if err != nil {
			return gui.CreateErrorPanel(fmt.Sprintf("%s: %s", gui.Tr.LcCommitPrefixPatternError, err.Error()))
		}
		prefix := rgx.ReplaceAllString(gui.getCheckedOutBranch().Name, prefixReplace)
		gui.renderString(gui.Views.CommitMessage, prefix)
		if err := gui.Views.CommitMessage.SetCursor(len(prefix), 0); err != nil {
			return err
		}
	}

	gui.g.Update(func(g *gocui.Gui) error {
		if err := gui.PushContext(gui.State.Contexts.CommitMessage); err != nil {
			return err
		}

		gui.RenderCommitLength()
		return nil
	})
	return nil
}

func (gui *Gui) promptToStageAllAndRetry(retry func() error) error {
	return gui.Ask(AskOpts{
		Title:  gui.Tr.NoFilesStagedTitle,
		Prompt: gui.Tr.NoFilesStagedPrompt,
		HandleConfirm: func() error {
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.StageAllFiles).StageAll(); err != nil {
				return gui.SurfaceError(err)
			}
			if err := gui.refreshFilesAndSubmodules(); err != nil {
				return gui.SurfaceError(err)
			}

			return retry()
		},
	})
}

func (gui *Gui) HandleAmendCommitPress() error {
	if gui.State.FileManager.GetItemsLength() == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoFilesStagedTitle)
	}

	if len(gui.stagedFiles()) == 0 {
		return gui.promptToStageAllAndRetry(gui.HandleAmendCommitPress)
	}

	if len(gui.State.Commits) == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoCommitToAmend)
	}

	return gui.Ask(AskOpts{
		Title:  strings.Title(gui.Tr.AmendLastCommit),
		Prompt: gui.Tr.SureToAmend,
		HandleConfirm: func() error {
			cmdStr := gui.GitCommand.AmendHeadCmdStr()
			gui.OnRunCommand(oscommands.NewCmdLogEntry(cmdStr, gui.Tr.Spans.AmendCommit, true))
			return gui.withGpgHandling(cmdStr, gui.Tr.AmendingStatus, nil)
		},
	})
}

// HandleCommitEditorPress - handle when the user wants to commit changes via
// their editor rather than via the popup panel
func (gui *Gui) HandleCommitEditorPress() error {
	if gui.State.FileManager.GetItemsLength() == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoFilesStagedTitle)
	}

	if len(gui.stagedFiles()) == 0 {
		return gui.promptToStageAllAndRetry(gui.HandleCommitEditorPress)
	}

	return gui.runSubprocessWithSuspenseAndRefresh(
		gui.OSCommand.WithSpan(gui.Tr.Spans.Commit).PrepareSubProcess("git", "commit"),
	)
}

func (gui *Gui) EditFile(filename string) error {
	cmdStr, err := gui.GitCommand.EditFileCmdStr(filename)
	if err != nil {
		return gui.SurfaceError(err)
	}

	return gui.runSubprocessWithSuspenseAndRefresh(
		gui.OSCommand.WithSpan(gui.Tr.Spans.EditFile).PrepareShellSubProcess(cmdStr),
	)
}

func (gui *Gui) HandleFileEdit() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	if node.File == nil {
		return gui.CreateErrorPanel(gui.Tr.ErrCannotEditDirectory)
	}

	return gui.EditFile(node.GetPath())
}

func (gui *Gui) HandleFileOpen() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	return gui.OpenFile(node.GetPath())
}

func (gui *Gui) HandleRefreshFiles() error {
	return gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{FILES}})
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

func (gui *Gui) handlePullFiles() error {
	if gui.popupPanelFocused() {
		return nil
	}

	span := gui.Tr.Spans.Pull

	currentBranch := gui.currentBranch()
	if currentBranch == nil {
		// need to wait for branches to refresh
		return nil
	}

	// if we have no upstream branch we need to set that first
	if currentBranch.Pullables == "?" {
		// see if we have this branch in our config with an upstream
		conf, err := gui.GitCommand.Repo.Config()
		if err != nil {
			return gui.SurfaceError(err)
		}
		for branchName, branch := range conf.Branches {
			if branchName == currentBranch.Name {
				return gui.pullFiles(PullFilesOptions{RemoteName: branch.Remote, BranchName: branch.Name, span: span})
			}
		}

		return gui.Prompt(PromptOpts{
			Title:          gui.Tr.EnterUpstream,
			InitialContent: "origin/" + currentBranch.Name,
			HandleConfirm: func(upstream string) error {
				if err := gui.GitCommand.SetUpstreamBranch(upstream); err != nil {
					errorMessage := err.Error()
					if strings.Contains(errorMessage, "does not exist") {
						errorMessage = fmt.Sprintf("upstream branch %s not found.\nIf you expect it to exist, you should fetch (with 'f').\nOtherwise, you should push (with 'shift+P')", upstream)
					}
					return gui.CreateErrorPanel(errorMessage)
				}
				return gui.pullFiles(PullFilesOptions{span: span})
			},
		})
	}

	return gui.pullFiles(PullFilesOptions{span: span})
}

type PullFilesOptions struct {
	RemoteName string
	BranchName string
	span       string
}

func (gui *Gui) pullFiles(opts PullFilesOptions) error {
	if err := gui.CreateLoaderPanel(gui.Tr.PullWait); err != nil {
		return err
	}

	mode := gui.Config.GetUserConfig().Git.Pull.Mode

	// TODO: this doesn't look like a good idea. Why the goroutine?
	go utils.Safe(func() { _ = gui.pullWithMode(mode, opts) })

	return nil
}

func (gui *Gui) pullWithMode(mode string, opts PullFilesOptions) error {
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

func (gui *Gui) pushWithForceFlag(force bool, upstream string, args string) error {
	if err := gui.CreateLoaderPanel(gui.Tr.PushWait); err != nil {
		return err
	}
	go utils.Safe(func() {
		branchName := gui.getCheckedOutBranch().Name
		err := gui.GitCommand.WithSpan(gui.Tr.Spans.Push).Push(branchName, force, upstream, args, gui.PromptUserForCredential)
		if err != nil && !force && strings.Contains(err.Error(), "Updates were rejected") {
			forcePushDisabled := gui.Config.GetUserConfig().Git.DisableForcePushing
			if forcePushDisabled {
				_ = gui.CreateErrorPanel(gui.Tr.UpdatesRejectedAndForcePushDisabled)
				return
			}
			_ = gui.Ask(AskOpts{
				Title:  gui.Tr.ForcePush,
				Prompt: gui.Tr.ForcePushPrompt,
				HandleConfirm: func() error {
					return gui.pushWithForceFlag(true, upstream, args)
				},
			})
			return
		}
		gui.HandleCredentialsPopup(err)
		_ = gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC})
	})
	return nil
}

func (gui *Gui) pushFiles() error {
	if gui.popupPanelFocused() {
		return nil
	}

	// if we have pullables we'll ask if the user wants to force push
	currentBranch := gui.currentBranch()
	if currentBranch == nil {
		// need to wait for branches to refresh
		return nil
	}

	if currentBranch.Pullables == "?" {
		// see if we have this branch in our config with an upstream
		conf, err := gui.GitCommand.Repo.Config()
		if err != nil {
			return gui.SurfaceError(err)
		}
		for branchName, branch := range conf.Branches {
			if branchName == currentBranch.Name {
				return gui.pushWithForceFlag(false, "", fmt.Sprintf("%s %s", branch.Remote, branchName))
			}
		}

		if gui.GitCommand.PushToCurrent {
			return gui.pushWithForceFlag(false, "", "--set-upstream")
		} else {
			return gui.Prompt(PromptOpts{
				Title:          gui.Tr.EnterUpstream,
				InitialContent: "origin " + currentBranch.Name,
				HandleConfirm: func(response string) error {
					return gui.pushWithForceFlag(false, response, "")
				},
			})
		}
	} else if currentBranch.Pullables == "0" {
		return gui.pushWithForceFlag(false, "", "")
	}

	forcePushDisabled := gui.Config.GetUserConfig().Git.DisableForcePushing
	if forcePushDisabled {
		return gui.CreateErrorPanel(gui.Tr.ForcePushDisabled)
	}

	return gui.Ask(AskOpts{
		Title:  gui.Tr.ForcePush,
		Prompt: gui.Tr.ForcePushPrompt,
		HandleConfirm: func() error {
			return gui.pushWithForceFlag(true, "", "")
		},
	})
}

func (gui *Gui) HandleSwitchToMerge() error {
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

func (gui *Gui) HandleCustomCommand() error {
	return gui.Prompt(PromptOpts{
		Title: gui.Tr.CustomCommand,
		HandleConfirm: func(command string) error {
			gui.OnRunCommand(oscommands.NewCmdLogEntry(command, gui.Tr.Spans.CustomCommand, true))
			return gui.runSubprocessWithSuspenseAndRefresh(
				gui.OSCommand.PrepareShellSubProcess(command),
			)
		},
	})
}

func (gui *Gui) HandleCreateStashMenu() error {
	menuItems := []*menuItem{
		{
			displayString: gui.Tr.LcStashAllChanges,
			onPress: func() error {
				return gui.handleStashSave(gui.GitCommand.WithSpan(gui.Tr.Spans.StashAllChanges).StashSave)
			},
		},
		{
			displayString: gui.Tr.LcStashStagedChanges,
			onPress: func() error {
				return gui.handleStashSave(gui.GitCommand.WithSpan(gui.Tr.Spans.StashStagedChanges).StashSaveStagedChanges)
			},
		},
	}

	return gui.createMenu(gui.Tr.LcStashOptions, menuItems, createMenuOptions{showCancel: true})
}

func (gui *Gui) HandleStashChanges() error {
	return gui.handleStashSave(gui.GitCommand.StashSave)
}

func (gui *Gui) HandleCreateResetToUpstreamMenu() error {
	return gui.CreateResetMenu("@{upstream}")
}

func (gui *Gui) HandleToggleDirCollapsed() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	gui.State.FileManager.ToggleCollapsed(node.GetPath())

	if err := gui.postRefreshUpdate(gui.State.Contexts.Files); err != nil {
		gui.Log.Error(err)
	}

	return nil
}

func (gui *Gui) HandleToggleFileTreeView() error {
	// get path of currently selected file
	path := gui.GetSelectedPath()

	gui.State.FileManager.ToggleShowTree()

	// find that same node in the new format and move the cursor to it
	if path != "" {
		gui.State.FileManager.ExpandToPath(path)
		index, found := gui.State.FileManager.GetIndexForPath(path)
		if found {
			gui.filesListContext().GetPanelState().SetSelectedLineIdx(index)
		}
	}

	if ContextKey(gui.Views.Files.Context) == FILES_CONTEXT_KEY {
		if err := gui.State.Contexts.Files.HandleRender(); err != nil {
			return err
		}
		if err := gui.State.Contexts.Files.HandleFocus(); err != nil {
			return err
		}
	}

	return nil
}

func (gui *Gui) HandleOpenMergeTool() error {
	return gui.Ask(AskOpts{
		Title:  gui.Tr.MergeToolTitle,
		Prompt: gui.Tr.MergeToolPrompt,
		HandleConfirm: func() error {
			return gui.runSubprocessWithSuspenseAndRefresh(
				gui.OSCommand.ExecutableFromString(gui.GitCommand.OpenMergeToolCmd()),
			)
		},
	})
}

func (gui *Gui) handleStashSave(stashFunc func(message string) error) error {
	if len(gui.trackedFiles()) == 0 && len(gui.stagedFiles()) == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoTrackedStagedFilesStash)
	}

	return gui.Prompt(PromptOpts{
		Title: gui.Tr.StashChanges,
		HandleConfirm: func(stashComment string) error {
			if err := stashFunc(stashComment); err != nil {
				return gui.SurfaceError(err)
			}
			return gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{STASH, FILES}})
		},
	})
}
