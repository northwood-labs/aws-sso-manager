# Where should we write the GIF?
Output docs/vhs/default-profile.mp4

Require "aws-sso-manager"

# Style
Set WindowBar Colorful
# Set Theme "Builtin Dark"

Set Shell "bash"
Set FontSize 12
Set LineHeight 1.2
Set FontFamily "SF Mono"
Set Width 1400
Set Height 600
Set Padding 20
Set Margin 20
Set MarginFill "#fdf6e3"
Set BorderRadius 10

# Prompt set last after "Set" stagements.
Env PS1 "$ "
Env FZF_DEFAULT_OPTS ""

# Type a command in the terminal.
Type "# View help for the 'config' command."
Sleep 2s
Enter
Type "aws-sso-manager config --help"
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
Type "# View a list of all existing user settings."
Sleep 2s
Enter
Type "aws-sso-manager config get"
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
Type "# Get the default SSO profile to use for commands (currently empty)."
Sleep 2s
Enter
Type "aws-sso-manager config get profile-name"
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
Type "# Set the default SSO profile to use for commands."
Sleep 2s
Enter
Type "aws-sso-manager config set profile-name nwl"
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
Type "# Run a command without specifically adding an SSO profile."
Sleep 2s
Enter
Type "aws-sso-manager list"
Sleep 200ms
Enter
Sleep 5s
