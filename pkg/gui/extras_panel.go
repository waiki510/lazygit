package gui

func (gui *Gui) handleCreateExtrasMenuPanel() error {
	menuItems := []*menuItem{
		{
			displayString: gui.Tr.ToggleShowCommandLog,
			onPress: func() error {
				currentContext := gui.currentStaticContext()
				if gui.ShowExtrasWindow && currentContext.GetKey() == COMMAND_LOG_CONTEXT_KEY {
					if err := gui.returnFromContext(); err != nil {
						return err
					}
				}
				gui.ShowExtrasWindow = !gui.ShowExtrasWindow
				return nil
			},
		},
		{
			displayString: gui.Tr.FocusCommandLog,
			onPress: func() error {
				return gui.handleFocusCommandLog()
			},
		},
	}

	return gui.CreateMenu(gui.Tr.CommandLog, menuItems, CreateMenuOptions{ShowCancel: true})
}

func (gui *Gui) handleFocusCommandLog() error {
	gui.ShowExtrasWindow = true
	gui.State.Contexts.CommandLog.SetParentContext(gui.currentSideContext())
	return gui.PushContext(gui.State.Contexts.CommandLog)
}

func (gui *Gui) scrollUpExtra() error {
	gui.Views.Extras.Autoscroll = false

	return gui.scrollUpView(gui.Views.Extras)
}

func (gui *Gui) scrollDownExtra() error {
	gui.Views.Extras.Autoscroll = false

	if err := gui.scrollDownView(gui.Views.Extras); err != nil {
		return err
	}

	return nil
}
