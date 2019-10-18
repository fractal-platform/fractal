// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcserver contains implementations for net rpc server.
package rpcserver

import (
	"context"
	"fmt"
	"github.com/fractal-platform/fractal/rpc"
	"github.com/fractal-platform/fractal/utils/log"
	"reflect"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// callback is a method callback which was registered in the server
type callback struct {
	rcvr        reflect.Value  // receiver of method
	method      reflect.Method // callback
	argTypes    []reflect.Type // input argument types
	hasCtx      bool           // method's first argument is a context (not included in argTypes)
	errPos      int            // err return idx, of -1 when method cannot return error
	isSubscribe bool           // indication if the callback is a subscription
}

type callbacks map[string]*callback     // collection of RPC callbacks
type subscriptions map[string]*callback // collection of subscription callbacks

// service represents a registered object
type service struct {
	name          string        // name for service
	typ           reflect.Type  // receiver type
	callbacks     callbacks     // registered handlers
	subscriptions subscriptions // available subscriptions/notifications
}

type serviceHolder struct {
	services map[string]*service
}

// serverRequest is an incoming request
type serverRequest struct {
	id            interface{}
	svcname       string
	callb         *callback
	args          []reflect.Value
	isUnsubscribe bool
	err           Error
}

func newServiceHolder() *serviceHolder {
	return &serviceHolder{
		services: make(map[string]*service),
	}
}

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// formatName will convert to first character to lower case
func formatName(name string) string {
	ret := []rune(name)
	if len(ret) > 0 {
		ret[0] = unicode.ToLower(ret[0])
	}
	return string(ret)
}

var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// isContextType returns an indication if the given t is of context.Context or *context.Context type
func isContextType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t == contextType
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Implements this type the error interface
func isErrorType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Implements(errorType)
}

var subscriptionType = reflect.TypeOf((*Subscription)(nil)).Elem()

// isSubscriptionType returns an indication if the given t is of Subscription or *Subscription type
func isSubscriptionType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t == subscriptionType
}

// isPubSub tests whether the given method has as as first argument a context.Context
// and returns the pair (Subscription, error)
func isPubSub(methodType reflect.Type) bool {
	// numIn(0) is the receiver type
	if methodType.NumIn() < 2 || methodType.NumOut() != 2 {
		return false
	}

	return isContextType(methodType.In(1)) &&
		isSubscriptionType(methodType.Out(0)) &&
		isErrorType(methodType.Out(1))
}

// suitableCallbacks iterates over the methods of the given type. It will determine if a method satisfies the criteria
// for a RPC callback or a subscription callback and adds it to the collection of callbacks or subscriptions. See server
// documentation for a summary of these criteria.
func suitableCallbacks(rcvr reflect.Value, typ reflect.Type) (callbacks, subscriptions) {
	callbacks := make(callbacks)
	subscriptions := make(subscriptions)

METHODS:
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := formatName(method.Name)
		if method.PkgPath != "" { // method must be exported
			continue
		}

		var h callback
		h.isSubscribe = isPubSub(mtype)
		h.rcvr = rcvr
		h.method = method
		h.errPos = -1

		firstArg := 1
		numIn := mtype.NumIn()
		if numIn >= 2 && mtype.In(1) == contextType {
			h.hasCtx = true
			firstArg = 2
		}

		if h.isSubscribe {
			h.argTypes = make([]reflect.Type, numIn-firstArg) // skip rcvr type
			for i := firstArg; i < numIn; i++ {
				argType := mtype.In(i)
				if isExportedOrBuiltinType(argType) {
					h.argTypes[i-firstArg] = argType
				} else {
					continue METHODS
				}
			}

			subscriptions[mname] = &h
			continue METHODS
		}

		// determine method arguments, ignore first arg since it's the receiver type
		// Arguments must be exported or builtin types
		h.argTypes = make([]reflect.Type, numIn-firstArg)
		for i := firstArg; i < numIn; i++ {
			argType := mtype.In(i)
			if !isExportedOrBuiltinType(argType) {
				continue METHODS
			}
			h.argTypes[i-firstArg] = argType
		}

		// check that all returned values are exported or builtin types
		for i := 0; i < mtype.NumOut(); i++ {
			if !isExportedOrBuiltinType(mtype.Out(i)) {
				continue METHODS
			}
		}

		// when a method returns an error it must be the last returned value
		h.errPos = -1
		for i := 0; i < mtype.NumOut(); i++ {
			if isErrorType(mtype.Out(i)) {
				h.errPos = i
				break
			}
		}

		if h.errPos >= 0 && h.errPos != mtype.NumOut()-1 {
			continue METHODS
		}

		switch mtype.NumOut() {
		case 0, 1, 2:
			if mtype.NumOut() == 2 && h.errPos == -1 { // method must one return value and 1 error
				continue METHODS
			}
			callbacks[mname] = &h
		}
	}

	return callbacks, subscriptions
}

// Register will create a service for the given rcvr type under the given name.
func (holder *serviceHolder) register(name string, rcvr interface{}) error {
	svc := new(service)
	svc.typ = reflect.TypeOf(rcvr)
	rcvrVal := reflect.ValueOf(rcvr)

	// name can not be empty
	if name == "" {
		return fmt.Errorf("no service name for type %s", svc.typ.String())
	}
	if !isExported(reflect.Indirect(rcvrVal).Type().Name()) {
		return fmt.Errorf("%s is not exported", reflect.Indirect(rcvrVal).Type().Name())
	}

	methods, subscriptions := suitableCallbacks(rcvrVal, svc.typ)

	if len(methods) == 0 && len(subscriptions) == 0 {
		return fmt.Errorf("Service %T doesn't have any suitable methods/subscriptions to expose", rcvr)
	}
	log.Info("Register api", "namespace", name, "service", svc.typ.String(), "methods", len(methods), "subscriptions", len(subscriptions))

	// already a previous service register under given name, merge methods/subscriptions
	if regsvc, present := holder.services[name]; present {
		for _, m := range methods {
			regsvc.callbacks[formatName(m.method.Name)] = m
		}
		for _, s := range subscriptions {
			regsvc.subscriptions[formatName(s.method.Name)] = s
		}
		return nil
	}

	svc.name = name
	svc.callbacks, svc.subscriptions = methods, subscriptions

	holder.services[svc.name] = svc
	return nil
}

// serveRequest will reads requests from the codec, calls the RPC callback and
// writes the response to the given codec.
func (holder *serviceHolder) serveRequest(ctx context.Context, codec *jsonCodec, sub bool) {
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			log.Error(string(buf))
		}
	}()

	//	ctx, cancel := context.WithCancel(context.Background())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// if the codec supports notification include a notifier that callbacks can use
	// to send notification to clients. It is tied to the codec/connection. If the
	// connection is closed the notifier will stop and cancels all active subscriptions.
	if sub {
		ctx = context.WithValue(ctx, notifierKey{}, newNotifier(codec))
	}

	//
	for {
		// read request from codec
		req, err := holder.readRequest(codec)
		if err != nil {
			// If a parsing error occurred, send an error
			if err.Error() != "EOF" {
				log.Error(fmt.Sprintf("read error %v\n", err))
				codec.Write(codec.CreateErrorResponse(nil, err))
			}
			return
		}

		if !sub {
			// If a single shot request is executing, run and return immediately
			holder.exec(ctx, codec, req)
			return
		}

		go func(req *serverRequest) {
			holder.exec(ctx, codec, req)
		}(req)
	}
}

// readRequest requests the next request from the codec.
func (holder *serviceHolder) readRequest(codec *jsonCodec) (*serverRequest, Error) {
	req, err := codec.ReadRequestHeader()
	if err != nil {
		return nil, err
	}

	var request *serverRequest

	// verify requests
	var ok bool
	var svc *service

	if req.err != nil {
		request = &serverRequest{id: req.id, err: req.err}
		return request, nil
	}

	if req.isPubSub && strings.HasSuffix(req.method, rpc.UnsubscribeMethodSuffix) {
		request = &serverRequest{id: req.id, isUnsubscribe: true}
		argTypes := []reflect.Type{reflect.TypeOf("")} // expect subscription id as first arg
		if args, err := codec.ParseRequestArguments(argTypes, req.params); err == nil {
			request.args = args
		} else {
			request.err = &invalidParamsError{err.Error()}
		}
		return request, nil
	}

	if svc, ok = holder.services[req.service]; !ok { // rpc method isn't available
		request = &serverRequest{id: req.id, err: &methodNotFoundError{req.service, req.method}}
		return request, nil
	}

	if req.isPubSub { // eth_subscribe, r.method contains the subscription method name
		if callb, ok := svc.subscriptions[req.method]; ok {
			request = &serverRequest{id: req.id, svcname: svc.name, callb: callb}
			if req.params != nil && len(callb.argTypes) > 0 {
				argTypes := []reflect.Type{reflect.TypeOf("")}
				argTypes = append(argTypes, callb.argTypes...)
				if args, err := codec.ParseRequestArguments(argTypes, req.params); err == nil {
					request.args = args[1:] // first one is service.method name which isn't an actual argument
				} else {
					request.err = &invalidParamsError{err.Error()}
				}
			}
		} else {
			request = &serverRequest{id: req.id, err: &methodNotFoundError{req.service, req.method}}
		}
		return request, nil
	}

	if callb, ok := svc.callbacks[req.method]; ok { // lookup RPC method
		request = &serverRequest{id: req.id, svcname: svc.name, callb: callb}
		if req.params != nil && len(callb.argTypes) > 0 {
			if args, err := codec.ParseRequestArguments(callb.argTypes, req.params); err == nil {
				request.args = args
			} else {
				request.err = &invalidParamsError{err.Error()}
			}
		}
		return request, nil
	}

	request = &serverRequest{id: req.id, err: &methodNotFoundError{req.service, req.method}}

	return request, nil
}

// createSubscription will call the subscription callback and returns the subscription id or error.
func (holder *serviceHolder) createSubscription(ctx context.Context, c *jsonCodec, req *serverRequest) (ID, error) {
	// subscription have as first argument the context following optional arguments
	args := []reflect.Value{req.callb.rcvr, reflect.ValueOf(ctx)}
	args = append(args, req.args...)
	reply := req.callb.method.Func.Call(args)

	if !reply[1].IsNil() { // subscription creation failed
		return "", reply[1].Interface().(error)
	}

	return reply[0].Interface().(*Subscription).ID, nil
}

// handle executes a request and returns the response from the callback.
func (holder *serviceHolder) handle(ctx context.Context, codec *jsonCodec, req *serverRequest) (interface{}, func()) {
	if req.err != nil {
		return codec.CreateErrorResponse(&req.id, req.err), nil
	}

	if req.isUnsubscribe { // cancel subscription, first param must be the subscription id
		if len(req.args) >= 1 && req.args[0].Kind() == reflect.String {
			notifier, supported := NotifierFromContext(ctx)
			if !supported { // interface doesn't support subscriptions (e.g. http)
				return codec.CreateErrorResponse(&req.id, &callbackError{rpc.ErrNotificationsUnsupported.Error()}), nil
			}

			subid := ID(req.args[0].String())
			if err := notifier.unsubscribe(subid); err != nil {
				return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
			}

			return codec.CreateResponse(req.id, true), nil
		}
		return codec.CreateErrorResponse(&req.id, &invalidParamsError{"Expected subscription id as first argument"}), nil
	}

	if req.callb.isSubscribe {
		subid, err := holder.createSubscription(ctx, codec, req)
		if err != nil {
			return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
		}

		// active the subscription after the sub id was successfully sent to the client
		activateSub := func() {
			notifier, _ := NotifierFromContext(ctx)
			notifier.activate(subid, req.svcname)
		}

		return codec.CreateResponse(req.id, subid), activateSub
	}

	// regular RPC call, prepare arguments
	if len(req.args) != len(req.callb.argTypes) {
		rpcErr := &invalidParamsError{fmt.Sprintf("%s%s%s expects %d parameters, got %d",
			req.svcname, rpc.ServiceMethodSeparator, req.callb.method.Name,
			len(req.callb.argTypes), len(req.args))}
		return codec.CreateErrorResponse(&req.id, rpcErr), nil
	}

	arguments := []reflect.Value{req.callb.rcvr}
	if req.callb.hasCtx {
		arguments = append(arguments, reflect.ValueOf(ctx))
	}
	if len(req.args) > 0 {
		arguments = append(arguments, req.args...)
	}

	// execute RPC method and return result
	reply := req.callb.method.Func.Call(arguments)
	if len(reply) == 0 {
		return codec.CreateResponse(req.id, nil), nil
	}
	if req.callb.errPos >= 0 { // test if method returned an error
		if !reply[req.callb.errPos].IsNil() {
			e := reply[req.callb.errPos].Interface().(error)
			res := codec.CreateErrorResponse(&req.id, &callbackError{e.Error()})
			return res, nil
		}
	}
	return codec.CreateResponse(req.id, reply[0].Interface()), nil
}

// exec executes the given request and writes the result back using the codec.
func (holder *serviceHolder) exec(ctx context.Context, codec *jsonCodec, req *serverRequest) {
	var response interface{}
	var callback func()
	if req.err != nil {
		response = codec.CreateErrorResponse(&req.id, req.err)
	} else {
		response, callback = holder.handle(ctx, codec, req)
	}

	if err := codec.Write(response); err != nil {
		log.Error(fmt.Sprintf("%v\n", err))
		codec.Close()
	}

	// when request was a subscribe request this allows these subscriptions to be actived
	if callback != nil {
		callback()
	}
}
