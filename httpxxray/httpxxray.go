// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

import "github.com/gogama/httpx"

const (
	nilClientMsg       = "httpxxray: nil client"
	nilHandlerGroupMsg = "httpxxray: nil handler group"
)

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
