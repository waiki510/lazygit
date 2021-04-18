package gui

import (
	"fmt"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/commands/oscommands"
	"github.com/jesseduffield/lazygit/pkg/config"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

type IGuiSyncing interface {
	PullWithMode(string, PullFilesOptions) error
}

type IGuiGlobalController interface {
	IGuiCommon
	IGuiSyncing
	IGuiBranches
	IGuiCredentials
}

type GlobalController struct {
	IGuiGlobalController
	*GuiCore
}

func NewGlobalController(gui *Gui) *GlobalController {
	return &GlobalController{IGuiGlobalController: gui, GuiCore: gui.GuiCore}
}

func (gui *GlobalController) GetKeybindings(keybindingsConfig config.KeybindingConfig, getKey func(string) interface{}) []*Binding {
	return []*Binding{
		{
			ViewName:    "",
			Key:         getKey(keybindingsConfig.Universal.PushFiles),
			Handler:     gui.HandlePushFiles,
			Description: gui.Tr.LcPush,
		},
		{
			ViewName:    "",
			Key:         getKey(keybindingsConfig.Universal.PullFiles),
			Handler:     gui.HandlePullFiles,
			Description: gui.Tr.LcPull,
		},
		{
			ViewName:    "",
			Key:         getKey(keybindingsConfig.Universal.ExecuteCustomCommand),
			Handler:     gui.HandleCustomCommand,
			Description: gui.Tr.LcExecuteCustomCommand,
		},
	}
}

func (gui *GlobalController) HandleCustomCommand() error {
	return gui.Prompt(PromptOpts{
		Title: gui.Tr.CustomCommand,
		HandleConfirm: func(command string) error {
			gui.OnRunCommand(oscommands.NewCmdLogEntry(command, gui.Tr.Spans.CustomCommand, true))
			return gui.RunSubprocessWithSuspenseAndRefresh(
				gui.OSCommand.PrepareShellSubProcess(command),
			)
		},
	})
}

func (gui *GlobalController) HandlePullFiles() error {
	if gui.PopupPanelFocused() {
		return nil
	}

	span := gui.Tr.Spans.Pull

	currentBranch := gui.GetCurrentBranch()
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
				return gui.PullFiles(PullFilesOptions{RemoteName: branch.Remote, BranchName: branch.Name, span: span})
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
				return gui.PullFiles(PullFilesOptions{span: span})
			},
		})
	}

	return gui.PullFiles(PullFilesOptions{span: span})
}

func (gui *GlobalController) PullFiles(opts PullFilesOptions) error {
	if err := gui.CreateLoaderPanel(gui.Tr.PullWait); err != nil {
		return err
	}

	mode := gui.Config.GetUserConfig().Git.Pull.Mode

	// TODO: this doesn't look like a good idea. Why the goroutine?
	go utils.Safe(func() { _ = gui.PullWithMode(mode, opts) })

	return nil
}

func (gui *GlobalController) HandlePushFiles() error {
	if gui.PopupPanelFocused() {
		return nil
	}

	// if we have pullables we'll ask if the user wants to force push
	currentBranch := gui.GetCurrentBranch()
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

func (gui *GlobalController) pushWithForceFlag(force bool, upstream string, args string) error {
	if err := gui.CreateLoaderPanel(gui.Tr.PushWait); err != nil {
		return err
	}
	go utils.Safe(func() {
		branchName := gui.GetCheckedOutBranch().Name
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
