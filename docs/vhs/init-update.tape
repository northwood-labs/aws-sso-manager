# Where should we write the GIF?
Output docs/vhs/init-update.mp4

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

Type "# View the existing ~/.aws/config file (empty state)."
Sleep 2s
Enter
Type "bat ~/.aws/config"
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

Type "# Initialize the tool for a new AWS SSO session."
Sleep 2s
Enter
Type "aws-sso-manager init"
Sleep 200ms
Enter
Sleep 3s

Type "nwl"
Sleep 200ms
Enter
Sleep 3s

Type "https://northwood-labs.awsapps.com/start"
Sleep 200ms
Enter
Sleep 3s

Type "us-east-2"
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

Type "# View the initialized ~/.aws/config file."
Sleep 2s
Enter
Type "bat ~/.aws/config"
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

Type "# Update the ~/.aws/config file with all SSO accounts and roles you have access to."
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

Type "# View the updated ~/.aws/config file."
Sleep 2s
Enter
Type "bat ~/.aws/config"
Sleep 200ms
Enter
Sleep 3s

# The end
Type "q"
Sleep 1s
