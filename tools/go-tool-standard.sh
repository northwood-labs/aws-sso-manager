#!/usr/bin/env bash
set -euo pipefail

# @config-manager:start standard
[[ ! -f go.tools.mod ]] && go mod init -modfile go.tools.mod tools
echo "replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.9.4" >> go.tools.mod

go get -tool -modfile go.tools.mod github.com/antham/gommit@latest
go get -tool -modfile go.tools.mod github.com/charmbracelet/gum@latest
go get -tool -modfile go.tools.mod github.com/evilmartians/lefthook@latest
go get -tool -modfile go.tools.mod github.com/google/osv-scanner/v2/cmd/osv-scanner@latest
go get -tool -modfile go.tools.mod github.com/goph/licensei/cmd/licensei@latest
go get -tool -modfile go.tools.mod github.com/pelletier/go-toml/v2/cmd/jsontoml@latest
go get -tool -modfile go.tools.mod github.com/pelletier/go-toml/v2/cmd/tomljson@latest
go get -tool -modfile go.tools.mod github.com/pelletier/go-toml/v2/cmd/tomll@latest
go get -tool -modfile go.tools.mod github.com/psampaz/gothanks@latest
go get -tool -modfile go.tools.mod github.com/tomwright/dasel/v3/cmd/dasel@latest
go get -tool -modfile go.tools.mod github.com/trufflesecurity/driftwood@latest
# @config-manager:end standard
