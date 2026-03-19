# Commit operations — demonstrates copy hash, reset menu, and squash
# confirmation dialog. These are the actual per-commit keybindings
# (y=copy, X=reset menu, s=squash, f=fixup, d=drop).
#
# Usage: vhs assets/rebase.tape

Output assets/rebase.gif

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

Type "kommit"
Enter
Sleep 3s

# Navigate past WIP to a real commit (commit ops are blocked on WIP)
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Sleep 800ms

# Copy commit hash with 'y' — shows a toast notification
Type "y"
Sleep 2s

# Navigate to another commit
Down
Sleep 400ms
Down
Sleep 400ms
Sleep 500ms

# Open the reset menu with 'X' — shows soft/mixed/hard/nuke picker
Type "X"
Sleep 2.5s

# Cancel the reset menu
Escape
Sleep 1s

# Show squash confirm dialog with 's'
Type "s"
Sleep 2.5s

# Cancel the squash
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
