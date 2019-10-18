// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package types contains log utils for fractal project.
package log

import (
	"os"
)

// Lvl is a type for predefined log levels.
type Lvl int

// List of predefined log Levels
const (
	LvlCrit Lvl = iota
	LvlError
	LvlWarn
	LvlInfo
	LvlDebug
)

var (
	defaultLogger Logger
)

type Logger interface {
	// Debug Log
	Debug(msg string, ctx ...interface{})

	// Info Log
	Info(msg string, ctx ...interface{})

	// Warn Log
	Warn(msg string, ctx ...interface{})

	// Error Log
	Error(msg string, ctx ...interface{})

	// Crit Log
	Crit(msg string, ctx ...interface{})

	// Sub Logger
	NewSubLogger(ctx ...interface{}) Logger
}

type Lazy struct {
	Fn interface{}
}

func init() {
	// set default logger to stdout
	SetDefaultLogger(InitLog15Logger(LvlInfo, os.Stdout))
}

func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}

func Debug(msg string, ctx ...interface{}) {
	defaultLogger.Debug(msg, ctx...)
}

func Info(msg string, ctx ...interface{}) {
	defaultLogger.Info(msg, ctx...)
}

func Warn(msg string, ctx ...interface{}) {
	defaultLogger.Warn(msg, ctx...)
}

func Error(msg string, ctx ...interface{}) {
	defaultLogger.Error(msg, ctx...)
}

func Crit(msg string, ctx ...interface{}) {
	defaultLogger.Crit(msg, ctx...)
}

func NewSubLogger(ctx ...interface{}) Logger {
	return defaultLogger.NewSubLogger(ctx...)
}
