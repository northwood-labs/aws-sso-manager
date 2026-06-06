# Where should we write the GIF?
Output docs/vhs/list.mp4

Require "aws-sso-manager"

# Style
Set WindowBar Colorful

Set Shell "zsh"
Set FontSize 12
Set LineHeight 1.2
Set FontFamily "SF Mono"
Set Width 1400
Set Height 1100
Set Padding 20
Set Margin 20
Set MarginFill "#fdf6e3"
Set BorderRadius 10

# Prompt set last after "Set" stagements.
Env PS1 "$ "

# Type a command in the terminal.
Type "aws-sso-manager list --help"
Sleep 200ms
Enter
Sleep 5s

Hide
Type "clear"
Sleep 200ms
Enter
Sleep 200ms
Show
Sleep 200ms

# Type a command in the terminal.
Type "aws-sso-manager list"
Sleep 200ms
Enter
Sleep 3s
