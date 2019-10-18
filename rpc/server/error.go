// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcserver contains implementations for net rpc server.
package rpcserver

import (
	"fmt"

	"github.com/fractal-platform/fractal/rpc"
)

// Error contain an error code in addition to the message.
type Error interface {
	Error() string  // returns the message
	ErrorCode() int // returns the code
}

// request is for an unknown service
type methodNotFoundError struct {
	service string
	method  string
}

func (e *methodNotFoundError) ErrorCode() int { return -32601 }

func (e *methodNotFoundError) Error() string {
	return fmt.Sprintf("The method %s%s%s does not exist/is not available", e.service, rpc.ServiceMethodSeparator, e.method)
}

// received message isn't a valid request
type invalidRequestError struct{ message string }

func (e *invalidRequestError) ErrorCode() int { return -32600 }

func (e *invalidRequestError) Error() string { return e.message }

// received message is invalid
type invalidMessageError struct{ message string }

func (e *invalidMessageError) ErrorCode() int { return -32700 }

func (e *invalidMessageError) Error() string { return e.message }

// unable to decode supplied params, or an invalid number of parameters
type invalidParamsError struct{ message string }

func (e *invalidParamsError) ErrorCode() int { return -32602 }

func (e *invalidParamsError) Error() string { return e.message }

// logic error, callback returned an error
type callbackError struct{ message string }

func (e *callbackError) ErrorCode() int { return -32000 }

func (e *callbackError) Error() string { return e.message }
