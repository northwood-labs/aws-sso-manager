# Where should we write the GIF?
Output docs/vhs/auth.mp4

Require "aws-sso-manager"

# Style
Set WindowBar Colorful

Set Shell "bash"
Set FontSize 12
Set LineHeight 1.2
Set FontFamily "SF Mono"
Set Width 1400
Set Height 800
Set Padding 20
Set Margin 20
Set MarginFill "#fdf6e3"
Set BorderRadius 10

# Prompt set last after "Set" stagements.
Env PS1 "$ "

# Type a command in the terminal.
Type "# View help for the 'auth' command."
Sleep 2s
Enter
Type "aws-sso-manager auth --help"
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
Type "# Authenticate with your AWS account."
Sleep 2s
Enter
Type "aws-sso-manager auth nwl"
Sleep 200ms
Enter
Sleep 7s

# Keys:
#   Escape[@<time>] [number]        Press the Escape key
#   Backspace[@<time>] [number]     Press the Backspace key
#   Delete[@<time>] [number]        Press the Delete key
#   Insert[@<time>] [number]        Press the Insert key
#   Down[@<time>] [number]          Press the Down key
#   Enter[@<time>] [number]         Press the Enter key
#   Space[@<time>] [number]         Press the Space key
#   Tab[@<time>] [number]           Press the Tab key
#   Left[@<time>] [number]          Press the Left Arrow key
#   Right[@<time>] [number]         Press the Right Arrow key
#   Up[@<time>] [number]            Press the Up Arrow key
#   Down[@<time>] [number]          Press the Down Arrow key
#   PageUp[@<time>] [number]        Press the Page Up key
#   PageDown[@<time>] [number]      Press the Page Down key
#   Ctrl+<key>                      Press the Control key + <key> (e.g. Ctrl+C)
#
# Display:
#   Hide                            Hide the subsequent commands from the output
#   Show                            Show the subsequent commands in the output
#   ScrollUp[@<time>] [number]      Scroll terminal viewport up by rows
#   ScrollDown[@<time>] [number]    Scroll terminal viewport down by rows
