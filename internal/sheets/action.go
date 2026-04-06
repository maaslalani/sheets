package sheets

// Action represents a named operation that can be triggered by a keybinding.
type Action string

// Normal mode actions.
const (
	ActionQuit                Action = "quit"
	ActionCommandPrompt       Action = "command_prompt"
	ActionSearchForward       Action = "search_forward"
	ActionSearchBackward      Action = "search_backward"
	ActionRegisterPending     Action = "register_pending"
	ActionGotoPending         Action = "goto_pending"
	ActionGotoBottom          Action = "goto_bottom"
	ActionMoveLeft            Action = "move_left"
	ActionMoveRight           Action = "move_right"
	ActionMoveUp              Action = "move_up"
	ActionMoveDown            Action = "move_down"
	ActionMoveToFirstCol      Action = "move_to_first_col"
	ActionMoveToFirstNonempty Action = "move_to_first_nonempty_col"
	ActionMoveToLastNonempty  Action = "move_to_last_nonempty_col"
	ActionMoveToWindowTop     Action = "move_to_window_top"
	ActionMoveToWindowMiddle  Action = "move_to_window_middle"
	ActionMoveToWindowBottom  Action = "move_to_window_bottom"
	ActionScrollHalfUp        Action = "scroll_half_up"
	ActionScrollHalfDown      Action = "scroll_half_down"
	ActionAlignPending        Action = "align_pending"
	ActionEnterInsert         Action = "enter_insert"
	ActionEnterInsertStart    Action = "enter_insert_start"
	ActionChangeCell          Action = "change_cell"
	ActionDeletePending       Action = "delete_pending"
	ActionOpenColAfter        Action = "open_col_after"
	ActionOpenColBefore       Action = "open_col_before"
	ActionOpenRowBelow        Action = "open_row_below"
	ActionOpenRowAbove        Action = "open_row_above"
	ActionEnterSelect         Action = "enter_select"
	ActionEnterRowSelect      Action = "enter_row_select"
	ActionUndo                Action = "undo"
	ActionRedo                Action = "redo"
	ActionYank                Action = "yank"
	ActionCut                 Action = "cut"
	ActionPaste               Action = "paste"
	ActionSearchNext          Action = "search_next"
	ActionSearchPrev          Action = "search_prev"
	ActionRepeatChange        Action = "repeat_change"
	ActionMarkSet             Action = "mark_set"
	ActionMarkJump            Action = "mark_jump"
	ActionMarkJumpExact       Action = "mark_jump_exact"
	ActionToggleBold          Action = "toggle_bold"
	ActionJumpForward         Action = "jump_forward"
	ActionJumpBackward        Action = "jump_backward"
)

// Select mode actions (in addition to shared navigation actions).
const (
	ActionCopySelection         Action = "copy_selection"
	ActionCopySelectionRef      Action = "copy_selection_ref"
	ActionCutSelection          Action = "cut_selection"
	ActionDeleteSelection       Action = "delete_selection"
	ActionFormulaFromSelection  Action = "formula_from_selection"
	ActionToggleRowSelect       Action = "toggle_row_select"
	ActionToggleSelectionBold   Action = "toggle_selection_bold"
)

// Insert mode actions.
const (
	ActionCommitDown        Action = "commit_down"
	ActionCommitRight       Action = "commit_right"
	ActionCommitLeft        Action = "commit_left"
	ActionCommitUp          Action = "commit_up"
	ActionCursorLeft        Action = "cursor_left"
	ActionCursorRight       Action = "cursor_right"
	ActionCursorHome        Action = "cursor_home"
	ActionCursorEnd         Action = "cursor_end"
	ActionDeleteAtCursor    Action = "delete_at_cursor"
	ActionDeleteBeforeCursor Action = "delete_before_cursor"
	ActionDeleteToStart     Action = "delete_to_start"
	ActionDeleteToEnd       Action = "delete_to_end"
	ActionDeleteWordBefore  Action = "delete_word_before"
	ActionInsertSpace       Action = "insert_space"
	ActionInsertRunes       Action = "insert_runes"
)
