// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

import (
	"testing"

	"github.com/gogama/httpx"
	"github.com/stretchr/testify/assert"
)

func TestOnClient(t *testing.T) {
	t.Run("nil Client", func(t *testing.T) {
		assert.PanicsWithValue(t, nilClientMsg, func() {
			OnClient(nil, &NopLogger{})
		})
	})
	t.Run("nil Logger", func(t *testing.T) {
		cl := &httpx.Client{
			Handlers: &httpx.HandlerGroup{},
		}
		OnClient(cl, nil)
	})
	t.Run("client has nil Handlers", func(t *testing.T) {
		cl := &httpx.Client{}
		OnClient(cl, &NopLogger{})
		assert.NotNil(t, cl.Handlers)
	})
	t.Run("everything", func(t *testing.T) {
		cl := &httpx.Client{
			Handlers: &httpx.HandlerGroup{},
		}
		OnClient(cl, &NopLogger{})
	})
}

func TestOnHandlers(t *testing.T) {
	t.Run("nil HandlerGroup", func(t *testing.T) {
		assert.PanicsWithValue(t, nilHandlerGroupMsg, func() {
			OnHandlers(nil, &NopLogger{})
		})
	})
	t.Run("nil Logger", func(t *testing.T) {
		h := &httpx.HandlerGroup{}
		OnHandlers(h, nil)
	})
	t.Run("everything", func(t *testing.T) {
		h := &httpx.HandlerGroup{}
		OnHandlers(h, &NopLogger{})
	})
}
