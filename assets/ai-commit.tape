# AI commit message generation — stage files with Space, trigger AI with ctrl+g.
# Panel focus uses number keys (1/2/3), not arrow keys.
#
# Usage: vhs assets/ai-commit.tape

Output assets/ai-commit.gif

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

# Launch — WIP is first item in commit list (cursor 0)
Type "kommit"
Enter
Sleep 3s

# Focus the detail panel (panel 3) — shows WIP unstaged files
Type "3"
Sleep 1.5s

# Stage a file with Space (not 's' which is section-jump)
Space
Sleep 800ms

# Move down and stage another file
Down
Sleep 400ms
Space
Sleep 800ms

# Trigger AI commit message generation
Ctrl+g
Sleep 8s

# Show the generated message
Sleep 3s

# Exit
Escape
Sleep 1s
Type "q"
Sleep 500ms
