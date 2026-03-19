# Theme switching — live theme preview in settings dialog.
# Settings opens with ',', first field is Theme, Enter opens picker,
# j/k cycles with instant preview.
#
# Usage: vhs assets/themes.tape

Output assets/themes.gif

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

# Open settings dialog with ','
Type ","
Sleep 1.5s

# Theme is the first field — press Enter to open the theme picker
Enter
Sleep 1s

# Cycle through themes (j = down) — each triggers live preview
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Down
Sleep 1.2s
Sleep 1.5s

# Select this theme
Enter
Sleep 1.5s

# Close settings
Escape
Sleep 1.5s

# Quit
Type "q"
Sleep 500ms
