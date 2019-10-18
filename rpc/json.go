// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpc contains defines for rpc server and client.
package rpc

import (
	"encoding/json"
	"fmt"
)

const (
	JsonrpcVersion         = "2.0"
	ServiceMethodSeparator = "_"

	ContentType = "application/json"
)

type JsonRequest struct {
	Method  string          `json:"method"`
	Version string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Payload json.RawMessage `json:"params,omitempty"`
}

type JsonSuccessResponse struct {
	Version string      `json:"jsonrpc"`
	Id      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result"`
}

type JsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err *JsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

func (err *JsonError) ErrorCode() int {
	return err.Code
}

type JsonErrResponse struct {
	Version string      `json:"jsonrpc"`
	Id      interface{} `json:"id,omitempty"`
	Error   JsonError   `json:"error"`
}

type JsonSubscription struct {
	Subscription string      `json:"subscription"`
	Result       interface{} `json:"result,omitempty"`
}

type JsonNotification struct {
	Version string           `json:"jsonrpc"`
	Method  string           `json:"method"`
	Params  JsonSubscription `json:"params"`
}
