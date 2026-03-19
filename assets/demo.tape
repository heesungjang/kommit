# Hero demo — showcases the full Kommit experience.
# Three-panel layout, commit graph, WIP staging area, diff viewer,
# command palette, and settings dialog.
#
# Usage: vhs assets/demo.tape

Output assets/demo.gif

Require kommit

Set Shell "bash"
Set FontSize 13
Set FontFamily "Menlo"
Set Width 1400
Set Height 750
Set Padding 12
Set Framerate 30
Set LetterSpacing 0
Set LineHeight 1.0
Set PlaybackSpeed 1.5
Set TypingSpeed 40ms
Set Theme { "name": "Kanagawa Wave", "black": "#2a2a37", "red": "#e46876", "green": "#98bb6c", "yellow": "#e6c384", "blue": "#7e9cd8", "magenta": "#957fb8", "cyan": "#7fb4ca", "white": "#dcd7ba", "brightBlack": "#54546d", "brightRed": "#e46876", "brightGreen": "#98bb6c", "brightYellow": "#e6c384", "brightBlue": "#7e9cd8", "brightMagenta": "#957fb8", "brightCyan": "#7fb4ca", "brightWhite": "#c8c093", "background": "#1f1f28", "foreground": "#dcd7ba", "selectionBackground": "#363646", "cursorColor": "#c8c093" }

Hide
Type "cd /Users/user1/Desktop/kommit-demo && clear"
Enter
Sleep 500ms
Show

# Launch kommit — starts with commit list focused, WIP row selected
Type "kommit"
Enter
Sleep 3s

# --- Scroll through commit graph ---
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Sleep 1.5s

# --- Focus sidebar (panel 1) to browse branches ---
Type "1"
Sleep 800ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Sleep 1.5s

# --- Jump back to commit list and go to WIP ---
Type "2"
Sleep 400ms
Type "g"
Sleep 1.5s

# --- Focus detail panel (panel 3) to show WIP staging area ---
Type "3"
Sleep 1.5s

# --- Open a file diff from unstaged files ---
Enter
Sleep 2s

# --- Scroll through diff, navigate hunks ---
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Sleep 800ms

Type "n"
Sleep 1.5s

# --- Close diff ---
Escape
Sleep 800ms

# --- Flash the command palette ---
Type "2"
Sleep 400ms
Ctrl+p
Sleep 2s
Escape
Sleep 1s

# --- Flash settings ---
Type ","
Sleep 2s
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
