package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"os"
)

func newProgramModel(args []string) (model, error) {
	m := newModel()
	if len(args) == 0 {
		return m, nil
	}

	if err := m.loadCSVFile(args[0]); err != nil {
		if os.IsNotExist(err) {
			m.currentFilePath = args[0]
			return m, nil
		}
		return model{}, err
	}

	return m, nil
}

func main() {
	m, err := newProgramModel(os.Args[1:])
	if err != nil {
		panic(err)
	}

	program := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		panic(err)
	}
}
