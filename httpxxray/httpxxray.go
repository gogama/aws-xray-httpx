// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

import "github.com/gogama/httpx"

const (
	nilClientMsg       = "httpxxray: nil client"
	nilHandlerGroupMsg = "httpxxray: nil handler group"
)

// OnClient installs X-Ray support onto an httpx Client.
//
// If client's current handler group is nil, OnClient creates a new
// handler group, sets it as client's current handler group, and
// proceeds to install X-Ray support into the handler group. If the
// handler group is not nil, OnClient adds X-Ray support into the
// existing handler group. (Be aware of this behavior if you are sharing
// a handler group among multiple clients.)
//
// Logger is used to log errors encountered by the plugin. The plugin
// does not produce any log messages in the ordinary course of operation
// and the logger is intended as a "just in case" debugging aid. To
// ignore errors, pass NopLogger (or nil, which is interpreted a
// NopLogger). However if you are using the plugin in a production
// system it is always prudent to use a viable logger.
func OnClient(client *httpx.Client, logger Logger) *httpx.Client {
	if client == nil {
		panic(nilClientMsg)
	}

	handlers := client.Handlers
	if handlers == nil {
		handlers = &httpx.HandlerGroup{}
		client.Handlers = handlers
	}

	OnHandlers(handlers, logger)

	return client
}

// OnHandlers installs X-Ray support onto an httpx HandlerGroup.
//
// The handler group may not be nil - if it is, a panic will ensue.
//
// Logger is used to log errors encountered by the plugin. The plugin
// does not produce any log messages in the ordinary course of operation
// and the logger is intended as a "just in case" debugging aid. To
// ignore errors, pass NopLogger (or nil, which is interpreted a
// NopLogger). However if you are using the plugin in a production
// system it is always prudent to use a viable logger.
func OnHandlers(handlers *httpx.HandlerGroup, logger Logger) *httpx.HandlerGroup {
	if handlers == nil {
		panic(nilHandlerGroupMsg)
	}

	if logger == nil {
		logger = NopLogger{}
	}

	handler := &handler{logger}
	handlers.PushBack(httpx.BeforeExecutionStart, handler)
	handlers.PushBack(httpx.BeforeAttempt, handler)
	handlers.PushBack(httpx.AfterAttempt, handler)
	handlers.PushBack(httpx.AfterPlanTimeout, handler)
	handlers.PushBack(httpx.AfterExecutionEnd, handler)

	return handlers
}
