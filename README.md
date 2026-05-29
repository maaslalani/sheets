# Sheets

Spreadsheets in your terminal.

<p align="center">
<img width="600" src="./examples/demo.gif?raw=true" alt="Sheets" />
</p>

## Features

- **Multi-format support:** Read and write `.csv`, `.tsv`, and `.md` (Markdown tables) natively.
- **Vim-inspired editing:** Navigate, yank, paste, and manipulate data without leaving your keyboard.
- **Built-in Formula Engine:** Evaluate math and aggregate data on the fly.
- **Interactive & CLI modes:** Use it as a full TUI or query specific cells directly from your shell.

## Usage

Launch `sheets` interactively:

```sh
> sheets budget.csv
````

Query or modify cells directly from the command line:

```sh
# Get the value of a specific cell
> sheets budget.csv B9

# Get a range of cells
> sheets budget.csv B1:B3

# Assign values directly
> sheets budget.csv B7=10 B8=20
```

Read from standard input:

```sh
> sheets <<< ID,Name,Age
1,Alice,24
2,Bob,32
3,Charlie,26
```

## Installation

You can build and install it directly from source:

```sh
go install github.com/maaslalani/sheets@latest
```

If you are on Arch Linux, you can install `sheets` from the AUR using your favorite helper:

```sh
# Install from source (git)
yay -S sheets-git

# Install pre-built binary
yay -S sheets-bin
```

Install with Homebrew on macOS or Linux:

```sh
brew install sheets
```

Or download a binary from the [releases](https://github.com/maaslalani/sheets/releases).

## Formulas & Functions

`sheets` features a robust internal engine to compute data. To use it, simply start a cell's content with `=` (e.g., `=A1+B1`).

### Supported Operators

- Arithmetic: `+`, `-`, `*`, `/`

### Aggregate Functions

You can aggregate data over ranges using the following supported functions:

- `SUM()`: Adds all numeric values in a range.
- `AVG()`: Calculates the average of numeric values.
- `MIN()`: Finds the lowest numeric value.
- `MAX()`: Finds the highest numeric value.
- `COUNT()`: Counts the number of numeric values.

### Shorthand References

Instead of typing full ranges, you can use powerful shorthands:

- **Full Column:** `=SUM(A)` calculates the sum of all cells in column A _up to the current row_.
- **Row Range:** `=AVG(1:3)` calculates the average of rows 1 through 3 _within the current column_.
- **Standard Range:** `=MAX(A1:B5)` computes over the defined 2D grid.

## Keybindings

### Normal Mode (Navigation & Editing)

|**Key**|**Action**|
|---|---|
|<kbd>h</kbd>, <kbd>j</kbd>, <kbd>k</kbd>, <kbd>l</kbd>|Move the active cell|
|<kbd>^</kbd>, <kbd>0</kbd> / <kbd>$</kbd>|Jump to the first column, first non-empty column, or last non-empty column in the row|
|<kbd>gg</kbd>, <kbd>G</kbd>, <kbd>5G</kbd>, <kbd>gB9</kbd>|Jump to the top, bottom, a row number, or a specific cell|
|<kbd>ctrl+d</kbd> / <kbd>ctrl+u</kbd>|Move half-page down / up|
|<kbd>H</kbd>, <kbd>M</kbd>, <kbd>L</kbd>|Jump to the top, middle, or bottom visible row|
|<kbd>zt</kbd>, <kbd>zz</kbd>, <kbd>zb</kbd>|Align the current row to the top, middle, or bottom of the window|
|<kbd>i</kbd> / <kbd>I</kbd>|Enter Insert mode / Insert at beginning of cell|
|<kbd>a</kbd> / <kbd>A</kbd>|Add column after / Add column before|
|<kbd>o</kbd> / <kbd>O</kbd>|Insert a row below or above and start editing|
|<kbd>c</kbd>|Change current cell (clears and enters Insert mode)|
|<kbd>x</kbd>|Cut current cell|
|<kbd>y</kbd> / <kbd>yy</kbd>|Yank (copy) current cell / row|
|<kbd>p</kbd>|Paste clipboard contents|
|<kbd>dd</kbd>|Delete current row|
|<kbd>u</kbd> / <kbd>ctrl+r</kbd>, <kbd>U</kbd>|Undo / Redo last operation|
|<kbd>.</kbd>|Repeat last change|
|<kbd>v</kbd> / <kbd>V</kbd>|Enter Visual mode / Line Visual mode|
|<kbd>enter</kbd>, <kbd>tab</kbd>, <kbd>shift+tab</kbd>|In insert mode, commit and move down, right, or left|
|<kbd>ctrl+n</kbd>, <kbd>ctrl+p</kbd>|In insert mode, commit and move down or up|
|<kbd>q</kbd>, <kbd>ctrl+c</kbd>|Quit|


### Visual Mode (Selection)

|**Key**|**Action**|
|---|---|
|<kbd>h</kbd>, <kbd>j</kbd>, <kbd>k</kbd>, <kbd>l</kbd>|Expand selection|
|<kbd>y</kbd>|Yank selection|
|<kbd>Y</kbd>|Yank selection reference (e.g. <kbd>A1:B3</kbd>)|
|<kbd>x</kbd>|Cut selection|
|<kbd>=</kbd>|In visual mode, insert a formula after the selected range `=|(B1:B8)`|
|<kbd>esc</kbd>|Exit Visual mode|


### Search & Marks

|**Key**|**Action**|
|---|---|
|<kbd>/</kbd> / <kbd>?</kbd>|Search forward / Search backward|
|<kbd>n</kbd> / <kbd>N</kbd>|Next search match / Previous search match|
|<kbd>m<char></kbd>|Set mark at current cell|
|<kbd>'<char></kbd>|Jump to mark|
|<kbd>ctrl+o</kbd> / <kbd>ctrl+i</kbd>|Jump backward / forward in jump list|


### Command Mode (<kbd>:</kbd>)

|**Command**|**Action**|
|---|---|
|<kbd>:w</kbd>|Write (save) current file|
|<kbd>:w <file></kbd>|Write to a new file|
|<kbd>:q</kbd>|Quit|
|<kbd>:q!</kbd>|Quit without saving|
|<kbd>:wq</kbd>|Write and quit|
|<kbd>:e <file></kbd>|Edit (load) a different file|
|<kbd>:goto <cell></kbd>|Jump to a specific cell (e.g., <kbd>:goto B5</kbd>)|
|<kbd>:help</kbd> or <kbd>:?</kbd>|Show basic command help|


## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.

## Feedback

I'd love to hear your feedback on improving `sheets`. Feel free to reach out via:

* [Email](mailto:maas@lalani.dev)
* [Twitter](https://twitter.com/maaslalani)
* [GitHub issues](https://github.com/maaslalani/sheets/issues/new)

---

<sub><sub>z</sub></sub><sub>z</sub>z
