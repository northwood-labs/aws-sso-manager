# Where should we write the GIF?
Output docs/vhs/lookup-get.mp4

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
Type "# View help for the 'lookup' command."
Sleep 2s
Enter
Type "aws-sso-manager lookup --help"
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
Type "# View help for the 'get' command."
Sleep 2s
Enter
Type "aws-sso-manager get --help"
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
Type "# View a list of AWS Account IDs that you have access to."
Sleep 2s
Enter
Type "aws-sso-manager get accounts"
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
Type "# Get the AWS Account ID for an account alias (matched by substring)."
Sleep 2s
Enter
Type "aws-sso-manager lookup account sandbox"
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
Type "# Get the list of roles for an account alias (matched by substring)."
Sleep 2s
Enter
Type "aws-sso-manager get roles --for $(aws-sso-manager lookup account sandbox)"
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
Type "# Get a single role (matched by substring) for an account alias."
Sleep 2s
Enter
Type "aws-sso-manager lookup role readonly --for $(aws-sso-manager lookup account sandbox)"
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
Type "# Get a single role (matched by substring) for an account alias,"
Enter
Type "# for an account that is selected using an 'fzf' menu."
Sleep 2s
Enter
Type "aws-sso-manager lookup role power --for $(aws-sso-manager get accounts | fzf)"
Sleep 200ms
Enter
Sleep 2s
Type@500ms "8903"
Sleep 200ms
Enter
Sleep 5s
