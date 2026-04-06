# Sheets

Spreadsheets in your terminal.

<br />
<p align="center">
<img width="800" src="./examples/demo.gif?raw=true" alt="Sheets" />
</p>
<br />

## Command Line Interface

Launch the TUI

```bash
> sheets budget.csv
```

Read from stdin:

```bash
> sheets <<< ID,Name,Age
1,Alice,24
2,Bob,32
3,Charlie,26
```

Read a specific cell:

```bash
> sheets budget.csv B9
2760
```

Or, range:

```bash
> sheets budget.csv B1:B3
1200
950
810
```

Modify a cell:

```bash
> sheets budget.csv B7=10 B8=20
```

## Navigation

* <kbd>h</kbd>, <kbd>j</kbd>, <kbd>k</kbd>, <kbd>l</kbd>: Move the active cell
* <kbd>gg</kbd>, <kbd>G</kbd>, <kbd>5G</kbd>, <kbd>gB9</kbd>: Jump to the top, bottom, a row number, or a specific cell
* <kbd>0</kbd>, <kbd>^</kbd>, <kbd>$</kbd>: Jump to the first column, first non-empty column, or last non-empty column in the row
* <kbd>H</kbd>, <kbd>M</kbd>, <kbd>L</kbd>: Jump to the top, middle, or bottom visible row
* <kbd>ctrl+u</kbd>, <kbd>ctrl+d</kbd>: Move half a page up or down
* <kbd>zt</kbd>, <kbd>zz</kbd>, <kbd>zb</kbd>,: Align the current row to the top, middle, or bottom of the window
* <kbd>/</kbd>, <kbd>?</kbd>: Search forward or backward
* <kbd>n</kbd>, <kbd>N</kbd>: Repeat the last search
* <kbd>ma</kbd>, <kbd>'a</kbd>: Set a mark and jump back to it later
* <kbd>ctrl+o</kbd>, <kbd>ctrl+i</kbd>: Move backward or forward through the jump list
* <kbd>q</kbd>, <kbd>ctrl+c</kbd>: Quit

### Editing & Selection

* <kbd>i</kbd>, <kbd>I</kbd>, <kbd>c</kbd>: Edit the current cell, edit from the start, or clear the cell and edit
* <kbd>ESC</kbd>: Leave insert, visual, or command mode
* <kbd>enter</kbd>, <kbd>tab</kbd>, <kbd>shift+tab</kbd>: In insert mode, commit and move down, right, or left
* <kbd>ctrl+n</kbd>, <kbd>ctrl+p</kbd>: In insert mode, commit and move down or up
* <kbd>o</kbd>, <kbd>O</kbd>: Insert a row below or above and start editing
* <kbd>v</kbd>, <kbd>V</kbd>: Start a visual selection or row selection
* <kbd>y</kbd>, <kbd>yy</kbd>: Copy the current cell, or yank the current row(s)
* <kbd>x</kbd>, <kbd>p</kbd>: Cut the current cell or selection, and paste the current register
* <kbd>dd</kbd>: Delete the current row
* <kbd>u</kbd>, <kbd>ctrl+r</kbd>, <kbd>U</kbd>: Undo and redo
* <kbd>.</kbd>: Repeat the last change

### Visual Mode

* <kbd>=</kbd>: In visual mode, insert a formula after the selected range `=|(B1:B8)`.

### Command Mode

Press <kbd>:</kbd> to open the command prompt, then use commands such as:

- <kbd>:w</kbd> to save
- <kbd>:w</kbd> <code>path.csv</code> to save to a new file
- <kbd>:e</kbd> <code>path.csv</code> to open another CSV
- <kbd>:q</kbd> or <kbd>:wq</kbd> to quit
- <kbd>:goto B9</kbd> or <kbd>:B9</kbd> to jump to a cell

## Configuration

Keybindings can be customized via a [Helix](https://docs.helix-editor.com/keymap.html)-style TOML config file at `~/.config/sheets/config.toml` (or `$XDG_CONFIG_HOME/sheets/config.toml`).

```toml
[keys.normal]
"C-c" = "quit"
"C-s" = "command_prompt"

[keys.insert]
"C-c" = "commit_down"

[keys.select]
"d" = "cut_selection"
```

Each section maps a key to an action name. Only specify the bindings you want to override — unset keys keep their defaults. Set a key to `"nop"` to disable it.

<details>
<summary>Available actions</summary>

#### Navigation

| Action | Description |
|--------|-------------|
| `move_left` | Move one cell left |
| `move_right` | Move one cell right |
| `move_up` | Move one cell up |
| `move_down` | Move one cell down |
| `move_to_first_col` | Jump to first column |
| `move_to_first_nonempty_col` | Jump to first non-empty column |
| `move_to_last_nonempty_col` | Jump to last non-empty column |
| `move_to_window_top` | Jump to top visible row |
| `move_to_window_middle` | Jump to middle visible row |
| `move_to_window_bottom` | Jump to bottom visible row |
| `scroll_half_up` | Scroll half page up |
| `scroll_half_down` | Scroll half page down |
| `goto_pending` | Start goto cell input |
| `goto_bottom` | Jump to last row |
| `jump_forward` | Jump list forward |
| `jump_backward` | Jump list backward |

#### Editing

| Action | Description |
|--------|-------------|
| `enter_insert` | Edit current cell |
| `enter_insert_start` | Edit from start of cell |
| `change_cell` | Clear cell and edit |
| `delete_pending` | Start delete row |
| `open_col_after` | Insert column after and edit |
| `open_col_before` | Insert column before and edit |
| `open_row_below` | Insert row below and edit |
| `open_row_above` | Insert row above and edit |
| `yank` | Copy cell |
| `cut` | Cut cell |
| `paste` | Paste |
| `undo` | Undo |
| `redo` | Redo |
| `repeat_change` | Repeat last change |
| `toggle_bold` | Toggle bold formatting |

#### Mode

| Action | Description |
|--------|-------------|
| `quit` | Quit |
| `command_prompt` | Open command prompt |
| `search_forward` | Search forward |
| `search_backward` | Search backward |
| `search_next` | Next search match |
| `search_prev` | Previous search match |
| `enter_select` | Enter visual select |
| `enter_row_select` | Enter row select |
| `register_pending` | Select register |
| `mark_set` | Set mark |
| `mark_jump` | Jump to mark |
| `mark_jump_exact` | Jump to mark (exact) |
| `align_pending` | Start alignment command |

#### Select mode

| Action | Description |
|--------|-------------|
| `copy_selection` | Copy selection |
| `copy_selection_ref` | Copy selection as reference |
| `cut_selection` | Cut selection |
| `formula_from_selection` | Insert formula from selection |
| `toggle_row_select` | Toggle row selection |
| `toggle_selection_bold` | Toggle bold on selection |

#### Insert mode

| Action | Description |
|--------|-------------|
| `commit_down` | Commit and move down |
| `commit_right` | Commit and move right |
| `commit_left` | Commit and move left |
| `commit_up` | Commit and move up |
| `cursor_left` | Move cursor left |
| `cursor_right` | Move cursor right |
| `cursor_home` | Move cursor to start |
| `cursor_end` | Move cursor to end |
| `delete_at_cursor` | Delete at cursor |
| `delete_before_cursor` | Delete before cursor |
| `delete_to_start` | Delete to start |
| `delete_to_end` | Delete to end |
| `delete_word_before` | Delete word before cursor |
| `insert_space` | Insert space |

</details>

**Key notation:** `C-x` for Ctrl+x, `S-tab` for Shift+Tab, `ret` for Enter, `space`, `backspace`, `del`, `esc`, `up`/`down`/`left`/`right`, `home`/`end`, `tab`. Literal characters like `h`, `j`, `:`, `/` are written as-is.

## Installation

<!--

Use a package manager:

```bash
# macOS
brew install sheets

# Arch
yay -S sheets

# Nix
nix-env -iA nixpkgs.sheets
```

-->

Install with Go:

```sh
go install github.com/maaslalani/sheets@main
```

Or download a binary from the [releases](https://github.com/maaslalani/sheets/releases).

## License

[MIT](https://github.com/maaslalani/sheets/blob/master/LICENSE)

## Feedback

I'd love to hear your feedback on improving `sheets`.

Feel free to reach out via:
* [Email](mailto:maas@lalani.dev)
* [Twitter](https://twitter.com/maaslalani)
* [GitHub issues](https://github.com/maaslalani/sheets/issues/new)

---

<sub><sub>z</sub></sub><sub>z</sub>z
