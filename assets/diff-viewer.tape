# Diff viewer — hunk navigation, fullscreen toggle, scrolling.
# Enter from detail panel (panel 3) opens diff in center panel.
#
# Usage: vhs assets/diff-viewer.tape

Output assets/diff-viewer.gif

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

# Navigate down to a commit with multiple file changes
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Sleep 800ms

# Focus detail panel (panel 3) to see commit file list
Type "3"
Sleep 1s

# Open file diff with Enter — diff loads in center panel
Enter
Sleep 2s

# Scroll through the diff
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
Sleep 1s

# Jump to next hunk
Type "n"
Sleep 1.5s

# Next hunk
Type "n"
Sleep 1.5s

# Toggle fullscreen diff
Type "f"
Sleep 2s

# Scroll in fullscreen
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 200ms
Sleep 1s

# Exit fullscreen
Type "f"
Sleep 1s

# Close diff viewer
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
