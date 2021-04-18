package gui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/gui/filetree"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

type IGuiBranches interface {
	GetCheckedOutBranch() *models.Branch
	GetCurrentBranch() *models.Branch
}

type IGuiCommitMessage interface {
	RenderCommitLength()
	WithGpgHandling(string, string, func() error) error
}

type IGuiFetching interface {
	Fetch(bool, string) error
}

type IGuiSideContext interface {
	HandleCopySelectedSideContextItemToClipboard() error
}

type IGuiFileChanges interface {
	GetSelectedFile() *models.File
	GetSelectedFileNode() *filetree.FileNode
	GetSelectedPath() string
	EnterFile(forceSecondaryFocused bool, selectedLineIdx int) error
	SwitchToMerge() error
	SelectFile(alreadySelected bool) error
	StagedFiles() []*models.File
	TrackedFiles() []*models.File
	RefreshFilesAndSubmodules() error
}

type IGuiSubmodules interface {
	ResetSubmodule(submodule *models.SubmoduleConfig) error
}

type IGuiFiles interface {
	EditFile(path string) error
	OpenFile(path string) error
}

type IGuiFilesController interface {
	IGuiCommon
	IGuiCredentials
	IGuiRefs
	IGuiFileChanges
	IGuiBranches
	IGuiCommitMessage
	IGuiFiles
	IGuiFetching
	IGuiSideContext
	IGuiSubmodules
}

type FilesController struct {
	IGuiFilesController
	*GuiCore
}

func NewFilesController(gui *Gui) *FilesController {
	return &FilesController{IGuiFilesController: gui, GuiCore: gui.GuiCore}
}

func (gui *FilesController) GetKeybindings(keybindingsConfig config.KeybindingConfig, getKey func(string) interface{}) []*Binding {
	return []*Binding{
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.CommitChanges),
			Handler:     gui.HandleCommitPress,
			Description: gui.Tr.CommitChanges,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.CommitChangesWithoutHook),
			Handler:     gui.HandleWIPCommitPress,
			Description: gui.Tr.LcCommitChangesWithoutHook,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.AmendLastCommit),
			Handler:     gui.HandleAmendCommitPress,
			Description: gui.Tr.AmendLastCommit,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.CommitChangesWithEditor),
			Handler:     gui.HandleCommitEditorPress,
			Description: gui.Tr.CommitChangesWithEditor,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.Select),
			Handler:     gui.HandleFilePress,
			Description: gui.Tr.LcToggleStaged,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.Remove),
			Handler:     gui.HandleCreateDiscardMenu,
			Description: gui.Tr.LcViewDiscardOptions,
			OpensMenu:   true,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.Edit),
			Handler:     gui.HandleFileEdit,
			Description: gui.Tr.LcEditFile,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.OpenFile),
			Handler:     gui.HandleFileOpen,
			Description: gui.Tr.LcOpenFile,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.IgnoreFile),
			Handler:     gui.HandleIgnoreFile,
			Description: gui.Tr.LcIgnoreFile,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.RefreshFiles),
			Handler:     gui.HandleRefreshFiles,
			Description: gui.Tr.LcRefreshFiles,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.StashAllChanges),
			Handler:     gui.HandleStashChanges,
			Description: gui.Tr.LcStashAllChanges,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.ViewStashOptions),
			Handler:     gui.HandleCreateStashMenu,
			Description: gui.Tr.LcViewStashOptions,
			OpensMenu:   true,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.ToggleStagedAll),
			Handler:     gui.HandleStageAll,
			Description: gui.Tr.LcToggleStagedAll,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.ViewResetOptions),
			Handler:     gui.HandleCreateResetMenu,
			Description: gui.Tr.LcViewResetOptions,
			OpensMenu:   true,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.GoInto),
			Handler:     gui.HandleEnterFile,
			Description: gui.Tr.FileEnter,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.Fetch),
			Handler:     gui.HandleGitFetch,
			Description: gui.Tr.LcFetch,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.CopyToClipboard),
			Handler:     gui.HandleCopyPathToClipboard,
			Description: gui.Tr.LcCopyFileNameToClipboard,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Commits.ViewResetOptions),
			Handler:     gui.HandleCreateResetToUpstreamMenu,
			Description: gui.Tr.LcViewResetToUpstreamOptions,
			OpensMenu:   true,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.ToggleTreeView),
			Handler:     gui.HandleToggleFileTreeView,
			Description: gui.Tr.LcToggleTreeView,
		},
		{
			ViewName:    "files",
			Contexts:    []string{string(FILES_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.OpenMergeTool),
			Handler:     gui.HandleOpenMergeTool,
			Description: gui.Tr.LcOpenMergeTool,
		},
		{
			ViewName:    "main",
			Contexts:    []string{string(MAIN_STAGING_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.CommitChanges),
			Handler:     gui.HandleCommitPress,
			Description: gui.Tr.CommitChanges,
		},
		{
			ViewName:    "main",
			Contexts:    []string{string(MAIN_STAGING_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.CommitChangesWithoutHook),
			Handler:     gui.HandleWIPCommitPress,
			Description: gui.Tr.LcCommitChangesWithoutHook,
		},
		{
			ViewName:    "main",
			Contexts:    []string{string(MAIN_STAGING_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.CommitChangesWithEditor),
			Handler:     gui.HandleCommitEditorPress,
			Description: gui.Tr.CommitChangesWithEditor,
		},
		{
			ViewName:    "main",
			Contexts:    []string{string(MAIN_MERGING_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Files.OpenMergeTool),
			Handler:     gui.HandleOpenMergeTool,
			Description: gui.Tr.LcOpenMergeTool,
		},
		{
			ViewName:    "main",
			Contexts:    []string{string(MAIN_STAGING_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.Edit),
			Handler:     gui.HandleFileEdit,
			Description: gui.Tr.LcEditFile,
		},
		{
			ViewName:    "main",
			Contexts:    []string{string(MAIN_STAGING_CONTEXT_KEY)},
			Key:         getKey(keybindingsConfig.Universal.OpenFile),
			Handler:     gui.HandleFileOpen,
			Description: gui.Tr.LcOpenFile,
		},
	}
}

func (gui *FilesController) HandleEnterFile() error {
	return gui.EnterFile(false, -1)
}

func (gui *FilesController) HandleFilePress() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	if node.IsLeaf() {
		file := node.File

		if file.HasInlineMergeConflicts {
			return gui.SwitchToMerge()
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

func (gui *FilesController) HandleStageAll() error {
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

func (gui *FilesController) allFilesStaged() bool {
	for _, file := range gui.State.FileManager.GetAllFiles() {
		if file.HasUnstagedChanges {
			return false
		}
	}
	return true
}

func (gui *FilesController) HandleIgnoreFile() error {
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

func (gui *FilesController) HandleWIPCommitPress() error {
	skipHookPrefix := gui.Config.GetUserConfig().Git.SkipHookPrefix
	if skipHookPrefix == "" {
		return gui.CreateErrorPanel(gui.Tr.SkipHookPrefixNotConfigured)
	}

	if err := gui.Views.CommitMessage.SetEditorContent(skipHookPrefix); err != nil {
		return err
	}

	return gui.HandleCommitPress()
}

func (gui *FilesController) HandleCommitPress() error {
	if err := gui.prepareFilesForCommit(); err != nil {
		return gui.SurfaceError(err)
	}

	if gui.State.FileManager.GetItemsLength() == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoFilesStagedTitle)
	}

	if len(gui.StagedFiles()) == 0 {
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
		prefix := rgx.ReplaceAllString(gui.GetCheckedOutBranch().Name, prefixReplace)
		gui.RenderString(gui.Views.CommitMessage, prefix)
		if err := gui.Views.CommitMessage.SetCursor(len(prefix), 0); err != nil {
			return err
		}
	}

	gui.OnMainThread(func() error {
		if err := gui.PushContext(gui.State.Contexts.CommitMessage); err != nil {
			return err
		}

		gui.RenderCommitLength()
		return nil
	})
	return nil
}

func (gui *FilesController) prepareFilesForCommit() error {
	noStagedFiles := len(gui.StagedFiles()) == 0
	if noStagedFiles && gui.Config.GetUserConfig().Gui.SkipNoStagedFilesWarning {
		err := gui.GitCommand.WithSpan(gui.Tr.Spans.StageAllFiles).StageAll()
		if err != nil {
			return err
		}

		return gui.RefreshFilesAndSubmodules()
	}

	return nil
}

func (gui *FilesController) promptToStageAllAndRetry(retry func() error) error {
	return gui.Ask(AskOpts{
		Title:  gui.Tr.NoFilesStagedTitle,
		Prompt: gui.Tr.NoFilesStagedPrompt,
		HandleConfirm: func() error {
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.StageAllFiles).StageAll(); err != nil {
				return gui.SurfaceError(err)
			}
			if err := gui.RefreshFilesAndSubmodules(); err != nil {
				return gui.SurfaceError(err)
			}

			return retry()
		},
	})
}

func (gui *FilesController) commitPrefixConfigForRepo() *config.CommitPrefixConfig {
	cfg, ok := gui.Config.GetUserConfig().Git.CommitPrefixes[utils.GetCurrentRepoName()]
	if !ok {
		return nil
	}

	return &cfg
}

func (gui *FilesController) HandleAmendCommitPress() error {
	if gui.State.FileManager.GetItemsLength() == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoFilesStagedTitle)
	}

	if len(gui.StagedFiles()) == 0 {
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
			return gui.WithGpgHandling(cmdStr, gui.Tr.AmendingStatus, nil)
		},
	})
}

// HandleCommitEditorPress - handle when the user wants to commit changes via
// their editor rather than via the popup panel
func (gui *FilesController) HandleCommitEditorPress() error {
	if gui.State.FileManager.GetItemsLength() == 0 {
		return gui.CreateErrorPanel(gui.Tr.NoFilesStagedTitle)
	}

	if len(gui.StagedFiles()) == 0 {
		return gui.promptToStageAllAndRetry(gui.HandleCommitEditorPress)
	}

	return gui.RunSubprocessWithSuspenseAndRefresh(
		gui.OSCommand.WithSpan(gui.Tr.Spans.Commit).PrepareSubProcess("git", "commit"),
	)
}

func (gui *FilesController) HandleFileEdit() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	if node.File == nil {
		return gui.CreateErrorPanel(gui.Tr.ErrCannotEditDirectory)
	}

	return gui.EditFile(node.GetPath())
}

func (gui *FilesController) HandleFileOpen() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	return gui.OpenFile(node.GetPath())
}

func (gui *FilesController) HandleRefreshFiles() error {
	return gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{FILES}})
}

func (gui *FilesController) HandleStashChanges() error {
	return gui.handleStashSave(gui.GitCommand.StashSave)
}

func (gui *FilesController) HandleCreateResetToUpstreamMenu() error {
	return gui.CreateResetMenu("@{upstream}")
}

func (gui *FilesController) HandleToggleFileTreeView() error {
	// get path of currently selected file
	path := gui.GetSelectedPath()

	gui.State.FileManager.ToggleShowTree()

	// find that same node in the new format and move the cursor to it
	if path != "" {
		gui.State.FileManager.ExpandToPath(path)
		index, found := gui.State.FileManager.GetIndexForPath(path)
		if found {
			gui.State.Contexts.Files.GetPanelState().SetSelectedLineIdx(index)
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

func (gui *FilesController) HandleOpenMergeTool() error {
	return gui.Ask(AskOpts{
		Title:  gui.Tr.MergeToolTitle,
		Prompt: gui.Tr.MergeToolPrompt,
		HandleConfirm: func() error {
			return gui.RunSubprocessWithSuspenseAndRefresh(
				gui.OSCommand.ExecutableFromString(gui.GitCommand.OpenMergeToolCmd()),
			)
		},
	})
}

func (gui *FilesController) HandleCreateStashMenu() error {
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

	return gui.CreateMenu(gui.Tr.LcStashOptions, menuItems, CreateMenuOptions{ShowCancel: true})
}

func (gui *FilesController) handleStashSave(stashFunc func(message string) error) error {
	if len(gui.TrackedFiles()) == 0 && len(gui.StagedFiles()) == 0 {
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

func (gui *FilesController) HandleGitFetch() error {
	if err := gui.CreateLoaderPanel(gui.Tr.FetchWait); err != nil {
		return err
	}
	go utils.Safe(func() {
		err := gui.Fetch(true, "Fetch")
		gui.HandleCredentialsPopup(err)
		_ = gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC})
	})
	return nil
}

func (gui *FilesController) HandleCopyPathToClipboard() error {
	return gui.HandleCopySelectedSideContextItemToClipboard()
}
