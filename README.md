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
