// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcserver contains implementations for net rpc server.
package rpcserver

import (
	"bytes"
	"context"
	"encoding/json"
	"golang.org/x/net/websocket"
	"net/http"
)

// wsHandler handles the subscription request
type wsHandler struct {
	serviceHolder *serviceHolder
	wsServer      *websocket.Server
}

// newWsHandler create handler for the subscription requests
func newWsHandler(serviceHolder *serviceHolder) *wsHandler {
	websocketJsonCodec := websocket.Codec{
		// Marshal is the stock JSON marshaller used by the websocket library too.
		Marshal: func(v interface{}) ([]byte, byte, error) {
			msg, err := json.Marshal(v)
			return msg, websocket.TextFrame, err
		},
		// Unmarshal is a specialized unmarshaller to properly convert numbers.
		Unmarshal: func(msg []byte, payloadType byte, v interface{}) error {
			dec := json.NewDecoder(bytes.NewReader(msg))
			dec.UseNumber()

			return dec.Decode(v)
		},
	}

	return &wsHandler{
		serviceHolder: serviceHolder,
		wsServer: &websocket.Server{
			Handler: func(conn *websocket.Conn) {
				// Create a custom encode/decode pair to enforce payload size and number encoding
				conn.MaxPayloadBytes = maxRequestContentLength

				encoder := func(v interface{}) error {
					return websocketJsonCodec.Send(conn, v)
				}
				decoder := func(v interface{}) error {
					return websocketJsonCodec.Receive(conn, v)
				}
				codec := newCodec(conn, encoder, decoder)
				defer codec.Close()
				serviceHolder.serveRequest(context.Background(), codec, true)
			},
		},
	}
}

func (h *wsHandler) handleWs(w http.ResponseWriter, r *http.Request) {
	h.wsServer.ServeHTTP(w, r)
}
