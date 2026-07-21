# Where should we write the GIF?
Output docs/vhs/auth.mp4

Require "aws-sso-manager"

# Style
Set WindowBar Colorful

Set Shell "bash"
Set FontSize 18
Set LineHeight 1.2
Set FontFamily "SF Mono"
Set Width 1280
Set Height 800
Set Padding 20
Set Margin 20
Set MarginFill "#fdf6e3"
Set BorderRadius 10

# Prompt set last after "Set" stagements.
Env PS1 "$ "

# Type a command in the terminal.
Type "# Authenticate with your AWS account."
Sleep 2s
Enter
Type "aws-sso-manager auth nwl"
Sleep 200ms
Enter
Sleep 7s
