package gui

import "github.com/jesseduffield/lazygit/pkg/commands/models"

// this is a controller: it can't access tags directly. Or can it? It should be able to get but not set. But that's exactly what I'm doing here, setting it. but through a mutator which encapsulates the event.
func (gui *Gui) refreshTags() error {
	tags, err := gui.GitCommand.GetTags()
	if err != nil {
		return gui.SurfaceError(err)
	}

	gui.State.Tags = tags

	return gui.postRefreshUpdate(gui.State.Contexts.Tags)
}

func (gui *Gui) GetSelectedTag() *models.Tag {
	selectedLine := gui.State.Panels.Tags.SelectedLineIdx
	if selectedLine == -1 || len(gui.State.Tags) == 0 {
		return nil
	}

	return gui.State.Tags[selectedLine]
}
