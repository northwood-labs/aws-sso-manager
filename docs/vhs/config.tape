# Where should we write the GIF?
Output docs/vhs/config.mp4

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

Type "# View the ~/.aws/config file."
Sleep 2s
Enter
Type "bat ~/.aws/config"
Sleep 200ms
Enter
Sleep 3s

Type "q"
Sleep 1s

Hide
Type "clear"
Sleep 200ms
Enter
Sleep 200ms
Show
Sleep 200ms

Type "# View the list of all AWS SSO accounts and roles you have access to."
Sleep 2s
Enter
Type "aws-sso-manager list nwl"
Sleep 200ms
Enter
Sleep 3s

Hide
Type "clear"
Sleep 200ms
Enter
Sleep 200ms
Show
Sleep 200ms

Type "# Edit user settings to change the generated profile name."
Sleep 2s
Enter
Type "nano ~/.config/aws-sso-manager/config.toml"
Sleep 200ms
Enter
Sleep 3s

Down 5
Right 12
Type "partner"
Sleep 3s
Ctrl+X
Sleep 200ms
Type "Y"
Sleep 200ms
Enter
Sleep 3s

Hide
Type "clear"
Sleep 200ms
Enter
Sleep 200ms
Show
Sleep 200ms

Type "# Update the list of SSO profiles in your ~/.aws/config file."
Sleep 2s
Enter
Type "aws-sso-manager update nwl"
Sleep 200ms
Enter
Sleep 3s

Hide
Type "clear"
Sleep 200ms
Enter
Sleep 200ms
Show
Sleep 200ms

Type "# View the updated the list of SSO profiles."
Sleep 2s
Enter
Type "aws-sso-manager list nwl"
Sleep 200ms
Enter
Sleep 3s
