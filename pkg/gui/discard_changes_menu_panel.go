package gui

func (gui *FilesController) HandleCreateDiscardMenu() error {
	node := gui.GetSelectedFileNode()
	if node == nil {
		return nil
	}

	var menuItems []*menuItem
	if node.File == nil {
		menuItems = []*menuItem{
			{
				displayString: gui.Tr.LcDiscardAllChanges,
				onPress: func() error {
					if err := gui.GitCommand.WithSpan(gui.Tr.Spans.DiscardAllChangesInDirectory).DiscardAllDirChanges(node); err != nil {
						return gui.SurfaceError(err)
					}
					return gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC, Scope: []RefreshableView{FILES}})
				},
			},
		}

		if node.GetHasStagedChanges() && node.GetHasUnstagedChanges() {
			menuItems = append(menuItems, &menuItem{
				displayString: gui.Tr.LcDiscardUnstagedChanges,
				onPress: func() error {
					if err := gui.GitCommand.WithSpan(gui.Tr.Spans.DiscardUnstagedChangesInDirectory).DiscardUnstagedDirChanges(node); err != nil {
						return gui.SurfaceError(err)
					}

					return gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC, Scope: []RefreshableView{FILES}})
				},
			})
		}
	} else {
		file := node.File

		submodules := gui.State.Submodules
		if file.IsSubmodule(submodules) {
			submodule := file.SubmoduleConfig(submodules)

			menuItems = []*menuItem{
				{
					displayString: gui.Tr.LcSubmoduleStashAndReset,
					onPress: func() error {
						return gui.WithWaitingStatus(gui.Tr.LcResettingSubmoduleStatus, func() error {
							return gui.ResetSubmodule(submodule)
						})
					},
				},
			}
		} else {
			menuItems = []*menuItem{
				{
					displayString: gui.Tr.LcDiscardAllChanges,
					onPress: func() error {
						if err := gui.GitCommand.WithSpan(gui.Tr.Spans.DiscardAllChangesInFile).DiscardAllFileChanges(file); err != nil {
							return gui.SurfaceError(err)
						}
						return gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC, Scope: []RefreshableView{FILES}})
					},
				},
			}

			if file.HasStagedChanges && file.HasUnstagedChanges {
				menuItems = append(menuItems, &menuItem{
					displayString: gui.Tr.LcDiscardUnstagedChanges,
					onPress: func() error {
						if err := gui.GitCommand.WithSpan(gui.Tr.Spans.DiscardAllUnstagedChangesInFile).DiscardUnstagedFileChanges(file); err != nil {
							return gui.SurfaceError(err)
						}

						return gui.RefreshSidePanels(RefreshOptions{Mode: ASYNC, Scope: []RefreshableView{FILES}})
					},
				})
			}
		}
	}

	return gui.CreateMenu(node.GetPath(), menuItems, CreateMenuOptions{ShowCancel: true})
}
