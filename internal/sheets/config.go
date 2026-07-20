package sheets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	tea "github.com/charmbracelet/bubbletea"
)

// Keymap maps key strings (Helix notation) to actions.
type Keymap map[string]Action

// KeymapConfig holds per-mode keymaps.
type KeymapConfig struct {
	Normal Keymap
	Insert Keymap
	Select Keymap
}

// configFile is the TOML structure matching Helix-style [keys.*] tables.
type configFile struct {
	Keys struct {
		Normal map[string]string `toml:"normal"`
		Insert map[string]string `toml:"insert"`
		Select map[string]string `toml:"select"`
	} `toml:"keys"`
}

// LoadKeymapConfig loads defaults and merges user overrides from
// $XDG_CONFIG_HOME/sheets/config.toml (falls back to ~/.config/sheets/config.toml).
func LoadKeymapConfig() KeymapConfig {
	km := KeymapConfig{
		Normal: defaultNormalKeys(),
		Insert: defaultInsertKeys(),
		Select: defaultSelectKeys(),
	}

	path := configPath()
	if path == "" {
		return km
	}

	var cfg configFile
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return km
		}
		return km
	}

	if len(cfg.Keys.Normal) > 0 {
		km.Normal = mergeKeymap(km.Normal, toKeymap(cfg.Keys.Normal))
	}
	if len(cfg.Keys.Insert) > 0 {
		km.Insert = mergeKeymap(km.Insert, toKeymap(cfg.Keys.Insert))
	}
	if len(cfg.Keys.Select) > 0 {
		km.Select = mergeKeymap(km.Select, toKeymap(cfg.Keys.Select))
	}

	return km
}

func configPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	path := filepath.Join(dir, "sheets", "config.toml")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func toKeymap(m map[string]string) Keymap {
	km := make(Keymap, len(m))
	for k, v := range m {
		km[k] = Action(v)
	}
	return km
}

func mergeKeymap(base, overrides Keymap) Keymap {
	merged := make(Keymap, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		if v == "nop" {
			delete(merged, k)
		} else {
			merged[k] = v
		}
	}
	return merged
}

// keyToString converts a tea.KeyMsg to a Helix-style key string.
// Note: some bubbletea key constants share values (e.g. KeyTab == KeyCtrlI,
// KeyEnter == KeyCtrlJ, KeyBackspace == KeyCtrlH). We map to the canonical
// name and handle aliases via duplicate entries in the default keymaps.
func keyToString(msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyCtrlA:
		return "C-a"
	case tea.KeyCtrlB:
		return "C-b"
	case tea.KeyCtrlC:
		return "C-c"
	case tea.KeyCtrlD:
		return "C-d"
	case tea.KeyCtrlE:
		return "C-e"
	case tea.KeyCtrlF:
		return "C-f"
	case tea.KeyCtrlG:
		return "C-g"
	// tea.KeyCtrlH == tea.KeyBackspace — handled as "backspace"
	case tea.KeyBackspace:
		return "backspace"
	// tea.KeyCtrlI == tea.KeyTab — handled as "tab"
	case tea.KeyTab:
		return "tab"
	// tea.KeyCtrlJ == tea.KeyEnter — handled as "ret"
	case tea.KeyEnter:
		return "ret"
	case tea.KeyCtrlK:
		return "C-k"
	case tea.KeyCtrlL:
		return "C-l"
	case tea.KeyCtrlN:
		return "C-n"
	case tea.KeyCtrlO:
		return "C-o"
	case tea.KeyCtrlP:
		return "C-p"
	case tea.KeyCtrlR:
		return "C-r"
	case tea.KeyCtrlU:
		return "C-u"
	case tea.KeyCtrlW:
		return "C-w"
	case tea.KeyShiftTab:
		return "S-tab"
	case tea.KeyDelete:
		return "del"
	case tea.KeyEscape:
		return "esc"
	case tea.KeySpace:
		return "space"
	case tea.KeyUp:
		return "up"
	case tea.KeyDown:
		return "down"
	case tea.KeyLeft:
		return "left"
	case tea.KeyRight:
		return "right"
	case tea.KeyHome:
		return "home"
	case tea.KeyEnd:
		return "end"
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			return string(msg.Runes)
		}
		return fmt.Sprintf("%s", string(msg.Runes))
	}
	return msg.String()
}
