package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"syscall"
	"time"

	"github.com/fractal-platform/fractal/cmd/utils"
	"github.com/fractal-platform/fractal/ftl"
	"github.com/fractal-platform/fractal/utils/log"
	ftl_metrics "github.com/fractal-platform/fractal/utils/metrics"
	flock "github.com/prometheus/tsdb/fileutil"
	"github.com/rcrowley/go-metrics"
	"github.com/vrischmann/go-metrics-influxdb"
	"gopkg.in/urfave/cli.v1"
)

var (
	// Git SHA1 commit hash of the release (set via linker flags)
	gitCommit   = ""
	versionMeta = "unstable" // Version metadata to append to the version string
	// The app that holds all commands and flags.
	app = utils.NewApp(versionMeta, gitCommit, "the go-fractal command line interface")
)

func init() {
	// define help template
	cli.AppHelpTemplate = `{{.Name}} [options]

VERSION:
   {{.Version}}

{{if .Flags}}OPTIONS:
   {{range .Flags}}{{.}}
   {{end}}{{end}}
`

	// Initialize the CLI app and start gftl
	app.Action = gftl
	app.HideVersion = true // we have a command to print the version
	app.Copyright = "Copyright 2013-2019 The go-fractal Authors"
	app.Commands = []cli.Command{}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = append(app.Flags, configFileFlag, genesisAllocFlag, checkPointsFlag)
	app.Flags = append(app.Flags, generalFlags...)
	app.Flags = append(app.Flags, miningEnabledFlag, packEnabledFlag, packerIdFlag, unlockedAccountFlag)
	app.Flags = append(app.Flags, unlockCheckPointPriKeyFlag)
	app.Flags = append(app.Flags, rpcFlags...)
	app.Flags = append(app.Flags, networkFlags...)
	app.Flags = append(app.Flags, metricsFlags...)
	app.Flags = append(app.Flags, debugFlags...)

	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())

		//
		debug.SetGCPercent(50)

		// setup logger
		fp, err := os.Create("./node.log")
		if err != nil {
			return err
		}
		log.SetDefaultLogger(log.InitMultipleLog15Logger(log.Lvl(ctx.GlobalInt(verbosityFlag.Name)), fp, os.Stdout))

		// setup debug
		if err := DebugSetup(ctx); err != nil {
			return err
		}

		return nil
	}

	app.After = func(ctx *cli.Context) error {
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// gftl is the main entry point into the system if no special subcommand is ran.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func gftl(ctx *cli.Context) error {
	if args := ctx.Args(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}

	// metrics enabled
	if ctx.GlobalIsSet(metricsEnabledFlag.Name) {
		metrics.UseNilMetrics = false

		if ctx.GlobalIsSet(influxdbUrlFlag.Name) && ctx.GlobalIsSet(influxdbDatabaseFlag.Name) &&
			ctx.GlobalIsSet(influxdbUsernameFlag.Name) && ctx.GlobalIsSet(influxdbPasswordFlag.Name) {
			log.Info("start influxdb goroutine")
			go influxdb.InfluxDB(
				metrics.DefaultRegistry,                     // metrics registry
				time.Second*10,                              // interval
				ctx.GlobalString(influxdbUrlFlag.Name),      // the InfluxDB url
				ctx.GlobalString(influxdbDatabaseFlag.Name), // your InfluxDB database
				ctx.GlobalString(influxdbUsernameFlag.Name), // your InfluxDB user
				ctx.GlobalString(influxdbPasswordFlag.Name), // your InfluxDB password
			)
		}

		go ftl_metrics.CollectProcessMetrics()
	} else {
		metrics.UseNilMetrics = true
	}

	cfg := makeConfigNode(ctx)

	// Lock the instance directory to prevent concurrent use by another instance as well as
	// accidental use of the instance directory as a database.
	// TODO: seems not work
	instdir := filepath.Join(cfg.NodeConfig.DataDir, cfg.NodeConfig.Progname())
	if err := os.MkdirAll(instdir, 0700); err != nil {
		log.Error("mkdir failed", "dir", instdir, "error", err)
		return err
	}
	_, _, err := flock.Flock(filepath.Join(instdir, "LOCK"))
	if err != nil {
		log.Error("flock failed", "dir", instdir, "error", err)
		return err
	}

	// check unlock
	if ctx.GlobalBool(miningEnabledFlag.Name) {
		if ctx.GlobalString(unlockedAccountFlag.Name) == "" {
			utils.Fatalf("Can not mine without unlocking the account. Try to add --%s flag.", unlockedAccountFlag.Name)
		}
	}

	f, err := ftl.NewFtl(cfg)
	if err != nil {
		return err
	}
	f.Start()

	// wait
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigc)
	<-sigc
	fmt.Printf("Got interrupt, shutting down...\n")
	for i := 2; i > 0; i-- {
		<-sigc
		if i > 1 {
			fmt.Printf("Already shutting down, interrupt more to panic.\n")
		}
	}
	return nil
}
