#!/usr/bin/env bash
set -euo pipefail

##
# Only rely on these tools for things which are loaded by the partials which are
# also included for this language or type of project.
##

# @config-manager:start golang
go get -tool -modfile go.tools.mod github.com/davidrjenni/reftools/cmd/fillstruct@latest
go get -tool -modfile go.tools.mod github.com/davidrjenni/reftools/cmd/fillswitch@latest
go get -tool -modfile go.tools.mod github.com/davidrjenni/reftools/cmd/fixplurals@latest
go get -tool -modfile go.tools.mod github.com/fatih/gomodifytags@latest
go get -tool -modfile go.tools.mod github.com/mdempsky/unconvert@latest
go get -tool -modfile go.tools.mod github.com/nikolaydubina/go-binsize-treemap@latest
go get -tool -modfile go.tools.mod github.com/nikolaydubina/go-cover-treemap@latest
go get -tool -modfile go.tools.mod github.com/nikolaydubina/smrcptr@latest
go get -tool -modfile go.tools.mod github.com/orlangure/gocovsh@latest
go get -tool -modfile go.tools.mod github.com/quasilyte/go-consistent@latest
go get -tool -modfile go.tools.mod github.com/segmentio/golines@latest
go get -tool -modfile go.tools.mod golang.org/x/perf/cmd/benchstat@latest
go get -tool -modfile go.tools.mod golang.org/x/tools/cmd/godoc@latest
go get -tool -modfile go.tools.mod golang.org/x/tools/go/analysis/passes/fieldalignment@latest
go get -tool -modfile go.tools.mod golang.org/x/vuln/cmd/govulncheck@latest
go get -tool -modfile go.tools.mod gotest.tools/gotestsum@latest
go get -tool -modfile go.tools.mod mvdan.cc/gofumpt@latest
# @config-manager:end golang
