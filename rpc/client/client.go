// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcclient contains implementations for net rpc client.
package rpcclient

import (
	"bytes"
	"container/list"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/rpc"
	"github.com/fractal-platform/fractal/utils/log"

	"golang.org/x/net/websocket"
)

var (
	ErrClientQuit                = errors.New("client is closed")
	ErrNoResult                  = errors.New("no result in JSON-RPC response")
	ErrSubscriptionQueueOverflow = errors.New("subscription queue overflow")
)

const (
	// Timeouts
	tcpKeepAliveInterval = 30 * time.Second
	defaultDialTimeout   = 10 * time.Second // used when dialing if the context has no deadline
	defaultWriteTimeout  = 10 * time.Second // used for calls if the context has no deadline
	subscribeTimeout     = 5 * time.Second  // overall timeout eth_subscribe, rpc_modules calls
)

const (
	// Subscriptions are removed when the subscriber cannot keep up.
	//
	// This can be worked around by supplying a channel with sufficiently sized buffer,
	// but this can be inconvenient and hard to explain in the docs. Another issue with
	// buffered channels is that the buffer is static even though it might not be needed
	// most of the time.
	//
	// The approach taken here is to maintain a per-subscription linked list buffer
	// shrinks on demand. If the buffer reaches the size below, the subscription is
	// dropped.
	maxClientSubscriptionBuffer = 20000
)

// A value of this type can a JSON-RPC request, notification, successful response or
// error response. Which one it is depends on the fields.
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *rpc.JsonError  `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (msg *jsonrpcMessage) isNotification() bool {
	return msg.ID == nil && msg.Method != ""
}

func (msg *jsonrpcMessage) isResponse() bool {
	return msg.hasValidID() && msg.Method == "" && len(msg.Params) == 0
}

func (msg *jsonrpcMessage) hasValidID() bool {
	return len(msg.ID) > 0 && msg.ID[0] != '{' && msg.ID[0] != '['
}

func (msg *jsonrpcMessage) String() string {
	b, _ := json.Marshal(msg)
	return string(b)
}

// Client represents a connection to an RPC server.
type Client struct {
	idCounter   uint32
	connectFunc func(ctx context.Context) (net.Conn, error)
	isHTTP      bool

	// writeConn is only safe to access outside dispatch, with the
	// write lock held. The write lock is taken by sending on
	// requestOp and released by sending on sendDone.
	writeConn net.Conn

	// for dispatch
	close       chan struct{}
	didQuit     chan struct{}                  // closed when client quits
	reconnected chan net.Conn                  // where write/reconnect sends the new connection
	readErr     chan error                     // errors from read
	readResp    chan []*jsonrpcMessage         // valid messages from read
	requestOp   chan *requestOp                // for registering response IDs
	sendDone    chan error                     // signals write completion, releases write lock
	respWait    map[string]*requestOp          // active requests
	subs        map[string]*ClientSubscription // active subscriptions
}

type requestOp struct {
	ids  []json.RawMessage
	err  error
	resp chan *jsonrpcMessage // receives up to len(ids) responses
	sub  *ClientSubscription  // only set for EthSubscribe requests
}

func (op *requestOp) wait(ctx context.Context) (*jsonrpcMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-op.resp:
		return resp, op.err
	}
}

// Dial creates a new client for the given URL.
//
// The currently supported URL schemes are "http", "https", "ws" and "wss". If rawurl is a
// file name with no URL scheme, a local socket connection is established using UNIX
// domain sockets on supported platforms and named pipes on Windows. If you want to
// configure transport options, use DialHTTP, DialWebsocket or DialIPC instead.
//
// For websocket connections, the origin is set to the local host name.
//
// The client reconnects automatically if the connection is lost.
func Dial(rawurl string) (*Client, error) {
	return DialContext(context.Background(), rawurl)
}

// DialContext creates a new RPC client, just like Dial.
//
// The context is used to cancel or time out the initial connection establishment. It does
// not affect subsequent interactions with the client.
func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
		return DialHTTP(rawurl)
	case "ws", "wss":
		return DialWebsocket(ctx, rawurl, "")
	default:
		return nil, fmt.Errorf("no known transport for URL scheme %q", u.Scheme)
	}
}

// DialWebsocket creates a new RPC client that communicates with a JSON-RPC server
// that is listening on the given endpoint.
//
// The context is used for the initial connection establishment. It does not
// affect subsequent interactions with the client.
func DialWebsocket(ctx context.Context, endpoint, origin string) (*Client, error) {
	if origin == "" {
		var err error
		if origin, err = os.Hostname(); err != nil {
			return nil, err
		}
		if strings.HasPrefix(endpoint, "wss") {
			origin = "https://" + strings.ToLower(origin)
		} else {
			origin = "http://" + strings.ToLower(origin)
		}
	}
	config, err := websocket.NewConfig(endpoint + rpc.WebSocketPath, origin)
	if err != nil {
		return nil, err
	}

	return newClient(ctx, func(ctx context.Context) (net.Conn, error) {
		return wsDialContext(ctx, config)
	})
}

func wsDialContext(ctx context.Context, config *websocket.Config) (*websocket.Conn, error) {
	var conn net.Conn
	var err error
	switch config.Location.Scheme {
	case "ws":
		conn, err = dialContext(ctx, "tcp", wsDialAddress(config.Location))
	case "wss":
		dialer := contextDialer(ctx)
		conn, err = tls.DialWithDialer(dialer, "tcp", wsDialAddress(config.Location), config.TlsConfig)
	default:
		err = websocket.ErrBadScheme
	}
	if err != nil {
		return nil, err
	}
	ws, err := websocket.NewClient(config, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ws, err
}

var wsPortMap = map[string]string{"ws": "80", "wss": "443"}

func wsDialAddress(location *url.URL) string {
	if _, ok := wsPortMap[location.Scheme]; ok {
		if _, _, err := net.SplitHostPort(location.Host); err != nil {
			return net.JoinHostPort(location.Host, wsPortMap[location.Scheme])
		}
	}
	return location.Host
}

func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{KeepAlive: tcpKeepAliveInterval}
	return d.DialContext(ctx, network, addr)
}

func contextDialer(ctx context.Context) *net.Dialer {
	dialer := &net.Dialer{Cancel: ctx.Done(), KeepAlive: tcpKeepAliveInterval}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	} else {
		dialer.Deadline = time.Now().Add(defaultDialTimeout)
	}
	return dialer
}

var nullAddr, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:0")

type httpConn struct {
	client    *http.Client
	req       *http.Request
	closeOnce sync.Once
	closed    chan struct{}
}

// httpConn is treated specially by Client.
func (hc *httpConn) LocalAddr() net.Addr              { return nullAddr }
func (hc *httpConn) RemoteAddr() net.Addr             { return nullAddr }
func (hc *httpConn) SetReadDeadline(time.Time) error  { return nil }
func (hc *httpConn) SetWriteDeadline(time.Time) error { return nil }
func (hc *httpConn) SetDeadline(time.Time) error      { return nil }
func (hc *httpConn) Write([]byte) (int, error)        { panic("Write called") }

func (hc *httpConn) Read(b []byte) (int, error) {
	<-hc.closed
	return 0, io.EOF
}

func (hc *httpConn) Close() error {
	hc.closeOnce.Do(func() { close(hc.closed) })
	return nil
}

func (hc *httpConn) doRequest(ctx context.Context, msg interface{}) (io.ReadCloser, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req := hc.req.WithContext(ctx)
	req.Body = ioutil.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))

	resp, err := hc.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Body, errors.New(resp.Status)
	}
	return resp.Body, nil
}

// DialHTTPWithClient creates a new RPC client that connects to an RPC server over HTTP
// using the provided HTTP Client.
func DialHTTPWithClient(endpoint string, client *http.Client) (*Client, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint + rpc.RPCPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", rpc.ContentType)
	req.Header.Set("Accept", rpc.ContentType)

	initctx := context.Background()
	return newClient(initctx, func(context.Context) (net.Conn, error) {
		return &httpConn{client: client, req: req, closed: make(chan struct{})}, nil
	})
}

// DialHTTP creates a new RPC client that connects to an RPC server over HTTP.
func DialHTTP(endpoint string) (*Client, error) {
	return DialHTTPWithClient(endpoint, new(http.Client))
}

func newClient(initctx context.Context, connectFunc func(context.Context) (net.Conn, error)) (*Client, error) {
	conn, err := connectFunc(initctx)
	if err != nil {
		return nil, err
	}
	_, isHTTP := conn.(*httpConn)
	c := &Client{
		writeConn:   conn,
		isHTTP:      isHTTP,
		connectFunc: connectFunc,
		close:       make(chan struct{}),
		didQuit:     make(chan struct{}),
		reconnected: make(chan net.Conn),
		readErr:     make(chan error),
		readResp:    make(chan []*jsonrpcMessage),
		requestOp:   make(chan *requestOp),
		sendDone:    make(chan error, 1),
		respWait:    make(map[string]*requestOp),
		subs:        make(map[string]*ClientSubscription),
	}
	if !isHTTP {
		go c.dispatch(conn)
	}
	return c, nil
}

func (c *Client) sendHTTP(ctx context.Context, op *requestOp, msg interface{}) error {
	hc := c.writeConn.(*httpConn)
	respBody, err := hc.doRequest(ctx, msg)
	if respBody != nil {
		defer respBody.Close()
	}

	if err != nil {
		if respBody != nil {
			buf := new(bytes.Buffer)
			if _, err2 := buf.ReadFrom(respBody); err2 == nil {
				return fmt.Errorf("%v %v", err, buf.String())
			}
		}
		return err
	}
	var respmsg jsonrpcMessage
	if err := json.NewDecoder(respBody).Decode(&respmsg); err != nil {
		return err
	}
	op.resp <- &respmsg
	return nil
}

func (c *Client) nextID() json.RawMessage {
	id := atomic.AddUint32(&c.idCounter, 1)
	return []byte(strconv.FormatUint(uint64(id), 10))
}

// SupportedModules calls the rpc_modules method, retrieving the list of
// APIs that are available on the server.
func (c *Client) SupportedModules() (map[string]string, error) {
	var result map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), subscribeTimeout)
	defer cancel()
	err := c.CallContext(ctx, &result, "rpc_modules")
	return result, err
}

// Close closes the client, aborting any in-flight requests.
func (c *Client) Close() {
	if c.isHTTP {
		return
	}
	select {
	case c.close <- struct{}{}:
		<-c.didQuit
	case <-c.didQuit:
	}
}

// Call performs a JSON-RPC call with the given arguments and unmarshals into
// result if no error occurred.
//
// The result must be a pointer so that package json can unmarshal into it. You
// can also pass nil, in which case the result is ignored.
func (c *Client) Call(result interface{}, method string, args ...interface{}) error {
	ctx := context.Background()
	return c.CallContext(ctx, result, method, args...)
}

// CallContext performs a JSON-RPC call with the given arguments. If the context is
// canceled before the call has successfully returned, CallContext returns immediately.
//
// The result must be a pointer so that package json can unmarshal into it. You
// can also pass nil, in which case the result is ignored.
func (c *Client) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	msg, err := c.newMessage(method, args...)
	if err != nil {
		return err
	}
	op := &requestOp{ids: []json.RawMessage{msg.ID}, resp: make(chan *jsonrpcMessage, 1)}

	if c.isHTTP {
		err = c.sendHTTP(ctx, op, msg)
	} else {
		err = c.send(ctx, op, msg)
	}
	if err != nil {
		return err
	}

	// dispatch has accepted the request and will close the channel when it quits.
	switch resp, err := op.wait(ctx); {
	case err != nil:
		return err
	case resp.Error != nil:
		return resp.Error
	case len(resp.Result) == 0:
		return ErrNoResult
	default:
		return json.Unmarshal(resp.Result, &result)
	}
}

// Subscribe calls the "<namespace>_subscribe" method with the given arguments,
// registering a subscription. Server notifications for the subscription are
// sent to the given channel. The element type of the channel must match the
// expected type of content returned by the subscription.
//
// The context argument cancels the RPC request that sets up the subscription but has no
// effect on the subscription after Subscribe has returned.
//
// Slow subscribers will be dropped eventually. Client buffers up to 8000 notifications
// before considering the subscriber dead. The subscription Err channel will receive
// ErrSubscriptionQueueOverflow. Use a sufficiently large buffer on the channel or ensure
// that the channel usually has at least one reader to prevent this issue.
func (c *Client) Subscribe(ctx context.Context, namespace string, channel interface{}, args ...interface{}) (*ClientSubscription, error) {
	// Check type of channel first.
	chanVal := reflect.ValueOf(channel)
	if chanVal.Kind() != reflect.Chan || chanVal.Type().ChanDir()&reflect.SendDir == 0 {
		panic("first argument to Subscribe must be a writable channel")
	}
	if chanVal.IsNil() {
		panic("channel given to Subscribe must not be nil")
	}
	if c.isHTTP {
		return nil, rpc.ErrNotificationsUnsupported
	}

	msg, err := c.newMessage(namespace+rpc.SubscribeMethodSuffix, args...)
	if err != nil {
		return nil, err
	}
	op := &requestOp{
		ids:  []json.RawMessage{msg.ID},
		resp: make(chan *jsonrpcMessage),
		sub:  newClientSubscription(c, namespace, chanVal),
	}

	// Send the subscription request.
	// The arrival and validity of the response is signaled on sub.quit.
	if err := c.send(ctx, op, msg); err != nil {
		return nil, err
	}
	if _, err := op.wait(ctx); err != nil {
		return nil, err
	}
	return op.sub, nil
}

func (c *Client) newMessage(method string, paramsIn ...interface{}) (*jsonrpcMessage, error) {
	params, err := json.Marshal(paramsIn)
	if err != nil {
		return nil, err
	}
	return &jsonrpcMessage{Version: "2.0", ID: c.nextID(), Method: method, Params: params}, nil
}

// send registers op with the dispatch loop, then sends msg on the connection.
// if sending fails, op is deregistered.
func (c *Client) send(ctx context.Context, op *requestOp, msg interface{}) error {
	select {
	case c.requestOp <- op:
		log.Debug("", "msg", log.Lazy{Fn: func() string {
			return fmt.Sprint("sending ", msg)
		}})
		err := c.write(ctx, msg)
		c.sendDone <- err
		return err
	case <-ctx.Done():
		// This can happen if the client is overloaded or unable to keep up with
		// subscription notifications.
		return ctx.Err()
	case <-c.didQuit:
		return ErrClientQuit
	}
}

func (c *Client) write(ctx context.Context, msg interface{}) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultWriteTimeout)
	}
	// The previous write failed. Try to establish a new connection.
	if c.writeConn == nil {
		if err := c.reconnect(ctx); err != nil {
			return err
		}
	}
	c.writeConn.SetWriteDeadline(deadline)
	err := json.NewEncoder(c.writeConn).Encode(msg)
	if err != nil {
		c.writeConn = nil
	}
	return err
}

func (c *Client) reconnect(ctx context.Context) error {
	newconn, err := c.connectFunc(ctx)
	if err != nil {
		log.Debug(fmt.Sprintf("reconnect failed: %v", err))
		return err
	}
	select {
	case c.reconnected <- newconn:
		c.writeConn = newconn
		return nil
	case <-c.didQuit:
		newconn.Close()
		return ErrClientQuit
	}
}

// dispatch is the main loop of the client.
// It sends read messages to waiting calls to Call and BatchCall
// and subscription notifications to registered subscriptions.
func (c *Client) dispatch(conn net.Conn) {
	// Spawn the initial read loop.
	go c.read(conn)

	var (
		lastOp        *requestOp    // tracks last send operation
		requestOpLock = c.requestOp // nil while the send lock is held
		reading       = true        // if true, a read loop is running
	)
	defer close(c.didQuit)
	defer func() {
		c.closeRequestOps(ErrClientQuit)
		conn.Close()
		if reading {
			// Empty read channels until read is dead.
			for {
				select {
				case <-c.readResp:
				case <-c.readErr:
					return
				}
			}
		}
	}()

	for {
		select {
		case <-c.close:
			return

			// Read path.
		case batch := <-c.readResp:
			for _, msg := range batch {
				switch {
				case msg.isNotification():
					log.Debug("", "msg", log.Lazy{Fn: func() string {
						return fmt.Sprint("<-readResp: notification ", msg)
					}})
					c.handleNotification(msg)
				case msg.isResponse():
					log.Debug("", "msg", log.Lazy{Fn: func() string {
						return fmt.Sprint("<-readResp: response ", msg)
					}})
					c.handleResponse(msg)
				default:
					log.Debug("", "msg", log.Lazy{Fn: func() string {
						return fmt.Sprint("<-readResp: dropping weird message", msg)
					}})
					// TODO: maybe close
				}
			}

		case err := <-c.readErr:
			log.Debug("<-readErr", "err", err)
			c.closeRequestOps(err)
			conn.Close()
			reading = false

		case newconn := <-c.reconnected:
			log.Debug("<-reconnected", "reading", reading, "remote", conn.RemoteAddr())
			if reading {
				// Wait for the previous read loop to exit. This is a rare case.
				conn.Close()
				<-c.readErr
			}
			go c.read(newconn)
			reading = true
			conn = newconn

			// Send path.
		case op := <-requestOpLock:
			// Stop listening for further send ops until the current one is done.
			requestOpLock = nil
			lastOp = op
			for _, id := range op.ids {
				c.respWait[string(id)] = op
			}

		case err := <-c.sendDone:
			if err != nil {
				// Remove response handlers for the last send. We remove those here
				// because the error is already handled in Call or BatchCall. When the
				// read loop goes down, it will signal all other current operations.
				for _, id := range lastOp.ids {
					delete(c.respWait, string(id))
				}
			}
			// Listen for send ops again.
			requestOpLock = c.requestOp
			lastOp = nil
		}
	}
}

// closeRequestOps unblocks pending send ops and active subscriptions.
func (c *Client) closeRequestOps(err error) {
	didClose := make(map[*requestOp]bool)

	for id, op := range c.respWait {
		// Remove the op so that later calls will not close op.resp again.
		delete(c.respWait, id)

		if !didClose[op] {
			op.err = err
			close(op.resp)
			didClose[op] = true
		}
	}
	for id, sub := range c.subs {
		delete(c.subs, id)
		sub.quitWithError(err, false)
	}
}

func (c *Client) handleNotification(msg *jsonrpcMessage) {
	if !strings.HasSuffix(msg.Method, rpc.NotificationMethodSuffix) {
		log.Debug("dropping non-subscription message", "msg", msg)
		return
	}
	var subResult struct {
		ID     string          `json:"subscription"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(msg.Params, &subResult); err != nil {
		log.Debug("dropping invalid subscription message", "msg", msg)
		return
	}
	if c.subs[subResult.ID] != nil {
		c.subs[subResult.ID].deliver(subResult.Result)
	}
}

func (c *Client) handleResponse(msg *jsonrpcMessage) {
	op := c.respWait[string(msg.ID)]
	if op == nil {
		log.Debug("unsolicited response", "msg", msg)
		return
	}
	delete(c.respWait, string(msg.ID))
	// For normal responses, just forward the reply to Call/BatchCall.
	if op.sub == nil {
		op.resp <- msg
		return
	}
	// For subscription responses, start the subscription if the server
	// indicates success. EthSubscribe gets unblocked in either case through
	// the op.resp channel.
	defer close(op.resp)
	if msg.Error != nil {
		op.err = msg.Error
		return
	}
	if op.err = json.Unmarshal(msg.Result, &op.sub.subid); op.err == nil {
		go op.sub.start()
		c.subs[op.sub.subid] = op.sub
	}
}

// Reading happens on a dedicated goroutine.

func (c *Client) read(conn net.Conn) error {
	var (
		buf json.RawMessage
		dec = json.NewDecoder(conn)
	)
	readMessage := func() (rs []*jsonrpcMessage, err error) {
		buf = buf[:0]
		if err = dec.Decode(&buf); err != nil {
			return nil, err
		}
		rs = make([]*jsonrpcMessage, 1)
		err = json.Unmarshal(buf, &rs[0])
		return rs, err
	}

	for {
		resp, err := readMessage()
		if err != nil {
			c.readErr <- err
			return err
		}
		c.readResp <- resp
	}
}

// Subscriptions.

// A ClientSubscription represents a subscription established through EthSubscribe.
type ClientSubscription struct {
	client    *Client
	etype     reflect.Type
	channel   reflect.Value
	namespace string
	subid     string
	in        chan json.RawMessage

	quitOnce sync.Once     // ensures quit is closed once
	quit     chan struct{} // quit is closed when the subscription exits
	errOnce  sync.Once     // ensures err is closed once
	err      chan error
}

func newClientSubscription(c *Client, namespace string, channel reflect.Value) *ClientSubscription {
	sub := &ClientSubscription{
		client:    c,
		namespace: namespace,
		etype:     channel.Type().Elem(),
		channel:   channel,
		quit:      make(chan struct{}),
		err:       make(chan error, 1),
		in:        make(chan json.RawMessage),
	}
	return sub
}

// Err returns the subscription error channel. The intended use of Err is to schedule
// resubscription when the client connection is closed unexpectedly.
//
// The error channel receives a value when the subscription has ended due
// to an error. The received error is nil if Close has been called
// on the underlying client and no other error has occurred.
//
// The error channel is closed when Unsubscribe is called on the subscription.
func (sub *ClientSubscription) Err() <-chan error {
	return sub.err
}

// Unsubscribe unsubscribes the notification and closes the error channel.
// It can safely be called more than once.
func (sub *ClientSubscription) Unsubscribe() {
	sub.quitWithError(nil, true)
	sub.errOnce.Do(func() { close(sub.err) })
}

func (sub *ClientSubscription) quitWithError(err error, unsubscribeServer bool) {
	sub.quitOnce.Do(func() {
		// The dispatch loop won't be able to execute the unsubscribe call
		// if it is blocked on deliver. Close sub.quit first because it
		// unblocks deliver.
		close(sub.quit)
		if unsubscribeServer {
			sub.requestUnsubscribe()
		}
		if err != nil {
			if err == ErrClientQuit {
				err = nil // Adhere to subscription semantics.
			}
			sub.err <- err
		}
	})
}

func (sub *ClientSubscription) deliver(result json.RawMessage) (ok bool) {
	select {
	case sub.in <- result:
		return true
	case <-sub.quit:
		return false
	}
}

func (sub *ClientSubscription) start() {
	sub.quitWithError(sub.forward())
}

func (sub *ClientSubscription) forward() (err error, unsubscribeServer bool) {
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.quit)},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.in)},
		{Dir: reflect.SelectSend, Chan: sub.channel},
	}
	buffer := list.New()
	defer buffer.Init()
	for {
		var chosen int
		var recv reflect.Value
		if buffer.Len() == 0 {
			// Idle, omit send case.
			chosen, recv, _ = reflect.Select(cases[:2])
		} else {
			// Non-empty buffer, send the first queued item.
			cases[2].Send = reflect.ValueOf(buffer.Front().Value)
			chosen, recv, _ = reflect.Select(cases)
		}

		switch chosen {
		case 0: // <-sub.quit
			return nil, false
		case 1: // <-sub.in
			val, err := sub.unmarshal(recv.Interface().(json.RawMessage))
			if err != nil {
				return err, true
			}
			if buffer.Len() == maxClientSubscriptionBuffer {
				return ErrSubscriptionQueueOverflow, true
			}
			buffer.PushBack(val)
		case 2:                             // sub.channel<-
			cases[2].Send = reflect.Value{} // Don't hold onto the value.
			buffer.Remove(buffer.Front())
		}
	}
}

func (sub *ClientSubscription) unmarshal(result json.RawMessage) (interface{}, error) {
	val := reflect.New(sub.etype)
	err := json.Unmarshal(result, val.Interface())
	return val.Elem().Interface(), err
}

func (sub *ClientSubscription) requestUnsubscribe() error {
	var result interface{}
	return sub.client.Call(&result, sub.namespace+rpc.UnsubscribeMethodSuffix, sub.subid)
}
