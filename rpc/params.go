// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpc contains defines for rpc server and client.
package rpc

import "errors"

const (
	RPCPath       = "/rpc"
	WebSocketPath = "/ws"

	SubscribeMethodSuffix    = "_subscribe"
	UnsubscribeMethodSuffix  = "_unsubscribe"
	NotificationMethodSuffix = "_subscription"
)

var (
	// ErrNotificationsUnsupported is returned when the connection doesn't support notifications
	ErrNotificationsUnsupported = errors.New("notifications not supported")
	// ErrNotificationNotFound is returned when the notification for the given id is not found
	ErrSubscriptionNotFound = errors.New("subscription not found")
)
