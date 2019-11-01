// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package types contains log utils for fractal project.
package log

import (
	"io"
	"os"

	"github.com/inconshreveable/log15"
	"github.com/mattn/go-isatty"
)

type Log15Logger struct {
	origin log15.Logger
}

func processContext(ctx ...interface{}) {
	for i, item := range ctx {
		if value, ok := item.(Lazy); ok {
			// convert from log.Lazy to log15.Lazy
			ctx[i] = log15.Lazy{
				Fn: value.Fn,
			}
		}
	}
}

func (logger *Log15Logger) Debug(msg string, ctx ...interface{}) {
	processContext(ctx...)
	logger.origin.Debug(msg, ctx...)
}

func (logger *Log15Logger) Info(msg string, ctx ...interface{}) {
	processContext(ctx...)
	logger.origin.Info(msg, ctx...)
}

func (logger *Log15Logger) Warn(msg string, ctx ...interface{}) {
	processContext(ctx...)
	logger.origin.Warn(msg, ctx...)
}

func (logger *Log15Logger) Error(msg string, ctx ...interface{}) {
	processContext(ctx...)
	logger.origin.Error(msg, ctx...)
}

func (logger *Log15Logger) Crit(msg string, ctx ...interface{}) {
	processContext(ctx...)
	logger.origin.Crit(msg, ctx...)
}

func (logger *Log15Logger) NewSubLogger(ctx ...interface{}) Logger {
	return &Log15Logger{
		origin: logger.origin.New(ctx...),
	}
}

// convert log.Lvl to log15.Lvl
func convertLogLevel(level Lvl) log15.Lvl {
	var lvl log15.Lvl
	switch level {
	case LvlCrit:
		lvl = log15.LvlCrit
	case LvlError:
		lvl = log15.LvlError
	case LvlWarn:
		lvl = log15.LvlWarn
	case LvlInfo:
		lvl = log15.LvlInfo
	case LvlDebug:
		lvl = log15.LvlDebug
	default:
		lvl = log15.LvlInfo
	}
	return lvl
}

func InitLog15Logger(level Lvl, writer io.Writer) Logger {
	lvl := convertLogLevel(level)
	logger := &Log15Logger{
		origin: log15.Root(),
	}
	logger.origin.SetHandler(log15.LvlFilterHandler(lvl, log15.StreamHandler(writer, log15.LogfmtFormat())))
	return logger
}

func InitMultipleLog15Logger(level Lvl, fpWriter io.Writer, consoleFile *os.File) Logger {
	lvl := convertLogLevel(level)
	logger := &Log15Logger{
		origin: log15.Root(),
	}
	fpHandler := log15.LvlFilterHandler(lvl, log15.StreamHandler(fpWriter, log15.LogfmtFormat()))
	consoleHandler := log15.MatchFilterHandler("type", "console", log15.StreamHandler(consoleFile, log15.LogfmtFormat()))
	if isatty.IsTerminal(consoleFile.Fd()) {
		consoleHandler = log15.MatchFilterHandler("type", "console", log15.StreamHandler(consoleFile, log15.TerminalFormat()))
	}
	logger.origin.SetHandler(log15.MultiHandler(fpHandler, consoleHandler))
	return logger
}
