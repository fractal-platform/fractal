// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcserver contains implementations for net rpc server.
package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/fractal-platform/fractal/rpc"
)

// rpcHandler handles the normal method-call request
type rpcHandler struct {
	serviceHolder *serviceHolder
}

// newRpcHandler create handler for the normal method-call requests
func newRpcHandler(serviceHolder *serviceHolder) *rpcHandler {
	return &rpcHandler{
		serviceHolder: serviceHolder,
	}
}

// handleRpc process the normal method-call request
func (h *rpcHandler) handleRpc(w http.ResponseWriter, r *http.Request) {
	// validate rpc request
	if code, err := validateRpcRequest(r); err != nil {
		http.Error(w, err.Error(), code)
		return
	}

	// All checks passed, create a codec that reads direct from the request body
	// untilEOF and writes the response to w and order the server to process a
	// single request.
	ctx := r.Context()
	ctx = context.WithValue(ctx, "remote", r.RemoteAddr)
	ctx = context.WithValue(ctx, "scheme", r.Proto)
	ctx = context.WithValue(ctx, "local", r.Host)
	if ua := r.Header.Get("User-Agent"); ua != "" {
		ctx = context.WithValue(ctx, "User-Agent", ua)
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		ctx = context.WithValue(ctx, "Origin", origin)
	}

	body := io.LimitReader(r.Body, maxRequestContentLength)
	codec := newJsonCodec(&httpReadWriteNopCloser{body, w})
	defer codec.Close()

	w.Header().Set("content-type", rpc.ContentType)
	h.serviceHolder.serveRequest(ctx, codec, false)
}

// validateRpcRequest returns a non-zero response code and error message if the request is invalid.
func validateRpcRequest(r *http.Request) (int, error) {
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		return http.StatusMethodNotAllowed, errors.New("method not allowed")
	}
	if r.ContentLength > maxRequestContentLength {
		err := fmt.Errorf("content length too large (%d>%d)", r.ContentLength, maxRequestContentLength)
		return http.StatusRequestEntityTooLarge, err
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("content-type"))
	if r.Method != http.MethodOptions && (err != nil || mt != rpc.ContentType) {
		err := fmt.Errorf("invalid content type, only %s is supported", rpc.ContentType)
		return http.StatusUnsupportedMediaType, err
	}
	return 0, nil
}

// httpReadWriteNopCloser wraps a io.Reader and io.Writer with a NOP Close method.
type httpReadWriteNopCloser struct {
	io.Reader
	io.Writer
}

// Close does nothing and returns always nil
func (t *httpReadWriteNopCloser) Close() error {
	return nil
}
