# Where should we write the GIF?
Output docs/vhs/console-url.mp4

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
Type "# View help for the 'console' command."
Sleep 2s
Enter
Type "aws-sso-manager console --help"
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
Type "# This feature allows you to specify which AWS account and IAM/SSO role the viewer needs."
Sleep 200ms
Enter
Type "# These links will verify that you have the correct access, and are great for runbooks and documentation."
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
Type "# If you have an AWS console URL on your clipboard, it will be used to fill the URL field."
Sleep 200ms
Enter
Type "# Otherwise, you can pass it as a parameter. We will use a URL for the S3 console."
Sleep 2s
Enter
Type "aws-sso-manager console https://533267280695-gm4gmcwm.us-east-2.console.aws.amazon.com/s3/home?region=us-east-2#"
Sleep 200ms
Enter
Sleep 2s

# nwl
Enter
Sleep 2s

# MSA
Down 2
Sleep 200ms
Enter
Sleep 2s

# Role
Enter
Sleep 5s
