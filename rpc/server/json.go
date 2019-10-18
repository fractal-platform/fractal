// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcserver contains implementations for net rpc server.
package rpcserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/fractal-platform/fractal/rpc"
	"github.com/fractal-platform/fractal/utils/log"
)

// request represents a raw incoming request
type request struct {
	service  string
	method   string
	id       interface{}
	isPubSub bool
	params   interface{}
	err      Error // invalid batch element
}

// jsonCodec reads and writes JSON-RPC messages to the underlying connection.
// jsonCodec also has support for parsing arguments and serializing results.
type jsonCodec struct {
	closer sync.Once        // close closed channel once
	closed chan interface{} // closed on Close

	decMu  sync.Mutex                // guards the decoder
	decode func(v interface{}) error // decoder to allow multiple transports
	encMu  sync.Mutex                // guards the encoder
	encode func(v interface{}) error // encoder to allow multiple transports

	rw io.ReadWriteCloser // connection
}

// newCodec creates a new jsonCodec with support for JSON-RPC 2.0 based on given encoding and decoding methods.
func newCodec(rwc io.ReadWriteCloser, encode, decode func(v interface{}) error) *jsonCodec {
	return &jsonCodec{
		closed: make(chan interface{}),
		encode: encode,
		decode: decode,
		rw:     rwc,
	}
}

// newJsonCodec creates a new jsonCodec with support for JSON-RPC 2.0.
func newJsonCodec(rwc io.ReadWriteCloser) *jsonCodec {
	enc := json.NewEncoder(rwc)
	dec := json.NewDecoder(rwc)
	dec.UseNumber()

	return &jsonCodec{
		closed: make(chan interface{}),
		encode: enc.Encode,
		decode: dec.Decode,
		rw:     rwc,
	}
}

// ReadRequestHeaders will read new request without parsing the arguments.
// It will return request, and an indication of error when the incoming message could not be read/parsed.
func (c *jsonCodec) ReadRequestHeader() (*request, Error) {
	c.decMu.Lock()
	defer c.decMu.Unlock()

	var incomingMsg json.RawMessage
	if err := c.decode(&incomingMsg); err != nil {
		return nil, &invalidRequestError{err.Error()}
	}
	return parseRequest(incomingMsg)
}

// checkReqId returns an error when the given reqId isn't valid for RPC method calls.
// valid id's are strings, numbers or null
func checkReqId(reqId json.RawMessage) error {
	if len(reqId) == 0 {
		return fmt.Errorf("missing request id")
	}
	if _, err := strconv.ParseFloat(string(reqId), 64); err == nil {
		return nil
	}
	var str string
	if err := json.Unmarshal(reqId, &str); err == nil {
		return nil
	}
	return fmt.Errorf("invalid request id")
}

// parseRequest will parse a single request from the given RawMessage.
func parseRequest(incomingMsg json.RawMessage) (*request, Error) {
	var in rpc.JsonRequest
	if err := json.Unmarshal(incomingMsg, &in); err != nil {
		return nil, &invalidMessageError{err.Error()}
	}

	if err := checkReqId(in.Id); err != nil {
		return nil, &invalidMessageError{err.Error()}
	}

	// subscribe are special, they will always use `subscribeMethod` as first param in the payload
	if strings.HasSuffix(in.Method, rpc.SubscribeMethodSuffix) {
		req := request{id: &in.Id, isPubSub: true}
		if len(in.Payload) > 0 {
			// first param must be subscription name
			var subscribeMethod [1]string
			if err := json.Unmarshal(in.Payload, &subscribeMethod); err != nil {
				log.Debug(fmt.Sprintf("Unable to parse subscription method: %v\n", err))
				return nil, &invalidRequestError{"Unable to parse subscription request"}
			}

			req.service, req.method = strings.TrimSuffix(in.Method, rpc.SubscribeMethodSuffix), subscribeMethod[0]
			req.params = in.Payload
			return &req, nil
		}
		return nil, &invalidRequestError{"Unable to parse subscription request"}
	}

	if strings.HasSuffix(in.Method, rpc.UnsubscribeMethodSuffix) {
		return &request{id: &in.Id, isPubSub: true,
			method: in.Method, params: in.Payload}, nil
	}

	elems := strings.Split(in.Method, rpc.ServiceMethodSeparator)
	if len(elems) != 2 {
		return nil, &methodNotFoundError{in.Method, ""}
	}

	// regular RPC call
	if len(in.Payload) == 0 {
		return &request{service: elems[0], method: elems[1], id: &in.Id}, nil
	}

	return &request{service: elems[0], method: elems[1], id: &in.Id, params: in.Payload}, nil
}

// ParseRequestArguments tries to parse the given params (json.RawMessage) with the given
// types. It returns the parsed values or an error when the parsing failed.
func (c *jsonCodec) ParseRequestArguments(argTypes []reflect.Type, params interface{}) ([]reflect.Value, Error) {
	if args, ok := params.(json.RawMessage); !ok {
		return nil, &invalidParamsError{"Invalid params supplied"}
	} else {
		return parsePositionalArguments(args, argTypes)
	}
}

// parsePositionalArguments tries to parse the given args to an array of values with the
// given types. It returns the parsed values or an error when the args could not be
// parsed. Missing optional arguments are returned as reflect.Zero values.
func parsePositionalArguments(rawArgs json.RawMessage, types []reflect.Type) ([]reflect.Value, Error) {
	// Read beginning of the args array.
	dec := json.NewDecoder(bytes.NewReader(rawArgs))
	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, &invalidParamsError{"non-array args"}
	}
	// Read args.
	args := make([]reflect.Value, 0, len(types))
	for i := 0; dec.More(); i++ {
		if i >= len(types) {
			return nil, &invalidParamsError{fmt.Sprintf("too many arguments, want at most %d", len(types))}
		}
		argval := reflect.New(types[i])
		if err := dec.Decode(argval.Interface()); err != nil {
			return nil, &invalidParamsError{fmt.Sprintf("invalid argument %d: %v", i, err)}
		}
		if argval.IsNil() && types[i].Kind() != reflect.Ptr {
			return nil, &invalidParamsError{fmt.Sprintf("missing value for required argument %d", i)}
		}
		args = append(args, argval.Elem())
	}
	// Read end of args array.
	if _, err := dec.Token(); err != nil {
		return nil, &invalidParamsError{err.Error()}
	}
	// Set any missing args to nil.
	for i := len(args); i < len(types); i++ {
		if types[i].Kind() != reflect.Ptr {
			return nil, &invalidParamsError{fmt.Sprintf("missing value for required argument %d", i)}
		}
		args = append(args, reflect.Zero(types[i]))
	}
	return args, nil
}

// CreateResponse will create a JSON-RPC success response with the given id and reply as result.
func (c *jsonCodec) CreateResponse(id interface{}, reply interface{}) interface{} {
	return &rpc.JsonSuccessResponse{Version: rpc.JsonrpcVersion, Id: id, Result: reply}
}

// CreateErrorResponse will create a JSON-RPC error response with the given id and error.
func (c *jsonCodec) CreateErrorResponse(id interface{}, err Error) interface{} {
	return &rpc.JsonErrResponse{Version: rpc.JsonrpcVersion, Id: id, Error: rpc.JsonError{Code: err.ErrorCode(), Message: err.Error()}}
}

// CreateNotification will create a JSON-RPC notification with the given subscription id and event as params.
func (c *jsonCodec) CreateNotification(subid, namespace string, event interface{}) interface{} {
	return &rpc.JsonNotification{Version: rpc.JsonrpcVersion, Method: namespace + rpc.NotificationMethodSuffix,
		Params: rpc.JsonSubscription{Subscription: subid, Result: event}}
}

// Write message to client
func (c *jsonCodec) Write(res interface{}) error {
	c.encMu.Lock()
	defer c.encMu.Unlock()

	return c.encode(res)
}

// Close the underlying connection
func (c *jsonCodec) Close() {
	c.closer.Do(func() {
		close(c.closed)
		c.rw.Close()
	})
}

// Closed returns a channel which will be closed when Close is called
func (c *jsonCodec) Closed() <-chan interface{} {
	return c.closed
}
