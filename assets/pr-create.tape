# PR section and command palette — browse the PR sidebar section
# and use ctrl+p to search available actions.
#
# Usage: vhs assets/pr-create.tape

Output assets/pr-create.gif

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

# Open command palette and search for PR-related actions
Ctrl+p
Sleep 1.5s

Type "pull request"
Sleep 2s

# Show results
Sleep 1.5s

# Close palette
Escape
Sleep 1s

# Focus sidebar and navigate to PULL REQUESTS section at bottom
Type "1"
Sleep 500ms
Type "G"
Sleep 1.5s

# Show the PR section
Sleep 2s

# Quit
Type "q"
Sleep 500ms
