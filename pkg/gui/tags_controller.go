package gui

import (
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/utils"
)

type IGuiCore interface {
	Prompt(PromptOpts) error
	Ask(AskOpts) error
	WithWaitingStatus(message string, f func() error) error
	SurfaceError(error) error
}

type IGuiCredentials interface {
	PromptUserForCredential(passOrUname string) string
	HandleCredentialsPopup(err error)
}

type IGuiRefs interface {
	HandleCheckoutRef(ref string, options HandleCheckoutRefOptions) error
	CreateResetMenu(ref string) error
}

type IGui interface {
	IGuiCore
	IGuiCredentials
	IGuiRefs
	GetSelectedTag() *models.Tag
	RefreshMainViews(RefreshMainOpts) error
	RefreshSidePanels(RefreshOptions) error
	PushContext(Context) error
}

type TagsController struct {
	IGui
	*GuiCore
}

func NewTagsController(gui *Gui) *TagsController {
	return &TagsController{IGui: gui, GuiCore: gui.GuiCore}
}

func (gui *TagsController) HandleCreate() error {
	return gui.Prompt(PromptOpts{
		Title: gui.Tr.CreateTagTitle,
		HandleConfirm: func(tagName string) error {
			// leaving commit SHA blank so that we're just creating the tag for the current commit
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.CreateLightweightTag).CreateLightweightTag(tagName, ""); err != nil {
				return gui.SurfaceError(err)
			}
			return gui.RefreshSidePanels(RefreshOptions{Scope: []RefreshableView{COMMITS, TAGS}, Then: func() {
				// find the index of the tag and set that as the currently selected line
				for i, tag := range gui.State.Tags {
					if tag.Name == tagName {
						gui.State.Panels.Tags.SelectedLineIdx = i
						if err := gui.State.Contexts.Tags.HandleRender(); err != nil {
							gui.Log.Error(err)
						}

						return
					}
				}
			},
			})
		},
	})
}

// tag-specific handlers
// view model would need to raise an event called 'tag selected', perhaps containing a tag. The listener would _be_ the main view, or the main context, and it would be able to render to itself.
func (gui *TagsController) HandleSelect() error {
	var task updateTask
	tag := gui.GetSelectedTag()
	if tag == nil {
		task = NewRenderStringTask("No tags")
	} else {
		cmd := gui.OSCommand.ExecutableFromString(
			gui.GitCommand.GetBranchGraphCmdStr(tag.Name),
		)
		task = NewRunCommandTask(cmd)
	}

	return gui.RefreshMainViews(RefreshMainOpts{
		Main: &ViewUpdateOpts{
			Title: "Tag",
			Task:  task,
		},
	})
}

func (gui *TagsController) WithSelectedTag(f func(tag *models.Tag) error) func() error {
	return func() error {
		tag := gui.GetSelectedTag()
		if tag == nil {
			return nil
		}

		return f(tag)
	}
}

func (gui *TagsController) HandleCheckout(tag *models.Tag) error {
	if err := gui.HandleCheckoutRef(tag.Name, HandleCheckoutRefOptions{Span: gui.Tr.Spans.CheckoutTag}); err != nil {
		return err
	}
	return gui.PushContext(gui.State.Contexts.Branches)
}

func (gui *TagsController) HandleDelete(tag *models.Tag) error {
	prompt := utils.ResolvePlaceholderString(
		gui.Tr.DeleteTagPrompt,
		map[string]string{
			"tagName": tag.Name,
		},
	)

	return gui.Ask(AskOpts{
		Title:  gui.Tr.DeleteTagTitle,
		Prompt: prompt,
		HandleConfirm: func() error {
			if err := gui.GitCommand.WithSpan(gui.Tr.Spans.DeleteTag).DeleteTag(tag.Name); err != nil {
				return gui.SurfaceError(err)
			}
			return gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC, Scope: []RefreshableView{COMMITS, TAGS}})
		},
	})
}

func (gui *TagsController) HandlePush(tag *models.Tag) error {
	title := utils.ResolvePlaceholderString(
		gui.Tr.PushTagTitle,
		map[string]string{
			"tagName": tag.Name,
		},
	)

	return gui.Prompt(PromptOpts{
		Title:          title,
		InitialContent: "origin",
		HandleConfirm: func(response string) error {
			return gui.WithWaitingStatus(gui.Tr.PushingTagStatus, func() error {
				err := gui.GitCommand.WithSpan(gui.Tr.Spans.PushTag).PushTag(response, tag.Name, gui.PromptUserForCredential)
				gui.HandleCredentialsPopup(err)

				return nil
			})
		},
	})
}

func (gui *TagsController) HandleCreateResetMenu(tag *models.Tag) error {
	return gui.CreateResetMenu(tag.Name)
}
