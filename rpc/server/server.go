// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcserver contains implementations for net rpc server.
package rpcserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fractal-platform/fractal/rpc"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/rs/cors"
)

const (
	maxRequestContentLength = 1024 * 128
)

// Server represents a RPC server
type Server struct {
	// http server
	httpServer *http.Server

	// callback handler for request
	reqHandler *reqHandler

	// where we provide the service functions
	serviceHolder *serviceHolder
}

func newCorsHandler(srv http.Handler, allowedOrigins []string) http.Handler {
	// disable CORS support if user has not specified a custom CORS configuration
	if len(allowedOrigins) == 0 {
		return srv
	}
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{http.MethodPost, http.MethodGet},
		MaxAge:         600,
		AllowedHeaders: []string{"*"},
	})
	return c.Handler(srv)
}

// NewServer create http server for rpc & websocket requests
func NewServer(cors []string, addr string) *Server {
	serviceHolder := newServiceHolder()
	reqHandler := &reqHandler{
		rpcHandler: newRpcHandler(serviceHolder),
		wsHandler:  newWsHandler(serviceHolder),
	}
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      newCorsHandler(reqHandler, cors),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return &Server{
		httpServer:    httpServer,
		reqHandler:    reqHandler,
		serviceHolder: serviceHolder,
	}
}

// RegisterApis register api for server
func (srv *Server) RegisterApis(apis []RpcApi) {
	for _, api := range apis {
		err := srv.serviceHolder.register(api.Namespace, api.Service)
		if err != nil {
			log.Error("Register Api failed", "namespace", api.Namespace, "service", api.Service, "err", err)
		}
	}
}

// ListenAndServe starts the request handler loop
func (srv *Server) ListenAndServe() {
	srv.httpServer.ListenAndServe()
}

// reqHandler encapsulate the handler for both rpc & websocket
type reqHandler struct {
	rpcHandler *rpcHandler
	wsHandler  *wsHandler
}

func (srv *Server) Shutdown() {
	srv.httpServer.Shutdown(context.Background())
}

// callback handler for http server
func (h *reqHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// we differs the rpc & websocket request by request uri
	if r.RequestURI == rpc.RPCPath {
		h.rpcHandler.handleRpc(w, r)
	} else if r.RequestURI == rpc.WebSocketPath {
		h.wsHandler.handleWs(w, r)
	} else {
		http.Error(w, fmt.Sprintf("uri %s not found", r.RequestURI), http.StatusNotFound)
	}
}
