// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// gtool implements the cmd tools for gftl
package main

import (
	"fmt"
	"github.com/fractal-platform/fractal/cmd/utils"
	"github.com/fractal-platform/fractal/utils/log"
	"gopkg.in/urfave/cli.v1"
	"os"
	"sort"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit   = ""
	versionMeta = "unstable" // Version metadata to append to the version string
	// The app that holds all commands and flags.
	app = utils.NewApp(versionMeta, gitCommit, "the gtool command line interface")
)

func init() {

	cli.AppHelpTemplate = `{{.Name}} command [command options] subcommand

VERSION:
   {{.Version}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}
`
	cli.SubcommandHelpTemplate = `NAME:
   {{.HelpName}} - {{if .Description}}{{.Description}}{{else}}{{.Usage}}{{end}}

USAGE:
   {{.HelpName}} [command options] subcommand

SUBCOMMANDS:{{range .VisibleCategories}}{{if .Name}}
   {{.Name}}:{{end}}{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}
{{end}}{{if .VisibleFlags}}
OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}
`

	// Initialize the CLI app
	app.Action = nil
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2013-2019 The go-fractal Authors"
	app.Commands = []cli.Command{
		keysCommand,
		gstateCommand,
		txCommand,
		stateCommand,
		adminCommand,
		blockCommand,
		packerCommand,
		dbCommand,
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = []cli.Flag{
		VerbosityFlag,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initLogger(ctx *cli.Context) {
	verbosity := ctx.GlobalInt(VerbosityFlag.Name)
	log.SetDefaultLogger(log.InitLog15Logger(log.Lvl(verbosity), os.Stdout))
}
