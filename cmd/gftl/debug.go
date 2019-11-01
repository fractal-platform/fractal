package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/fjl/memsize/memsizeui"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
	"gopkg.in/urfave/cli.v1"
)

var Memsize memsizeui.Handler

// Setup initializes profiling and logging based on the CLI flags.
// It should be called as early as possible in the program.
func DebugSetup(ctx *cli.Context) error {
	// pprof server
	if ctx.GlobalBool(pprofFlag.Name) {
		address := fmt.Sprintf("%s:%d", ctx.GlobalString(pprofAddrFlag.Name), ctx.GlobalInt(pprofPortFlag.Name))
		StartPProf(address)
	}
	return nil
}

func StartPProf(address string) {
	// Hook go-metrics into expvar on any /debug/metrics request, load all vars
	// from the registry into expvar, and execute regular expvar handler.
	exp.Exp(metrics.DefaultRegistry)
	http.Handle("/memsize/", http.StripPrefix("/memsize", &Memsize))
	log.Info("Starting pprof server", "addr", fmt.Sprintf("http://%s/debug/pprof", address))
	go func() {
		if err := http.ListenAndServe(address, nil); err != nil {
			log.Error("Failure in running pprof server", "err", err)
		}
	}()
}
