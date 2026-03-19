# Branch management — browse branches, tags, stash in sidebar,
# create a new branch. Uses "1" key for sidebar focus.
# Skips checkout to avoid dirty-tree failures.
#
# Usage: vhs assets/branches.tape

Output assets/branches.gif

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

# Focus sidebar (panel 1)
Type "1"
Sleep 1s

# Browse local branches
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
Down
Sleep 400ms
Sleep 1s

# Keep scrolling into REMOTE section
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
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Sleep 1s

# Keep going to TAGS section
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
Sleep 1s

# Go back to top
Type "g"
Sleep 500ms

# Move to first branch and create a new one with 'n'
Down
Sleep 400ms
Type "n"
Sleep 2s

# Type the new branch name
Type "feature/observability"
Sleep 1.5s

# Cancel — demo only
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
