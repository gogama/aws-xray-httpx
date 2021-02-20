// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

import (
	"testing"
	"time"

	"github.com/gogama/httpx/racing"

	"github.com/aws/aws-xray-sdk-go/xray"

	"github.com/stretchr/testify/require"

	"github.com/gogama/httpx/retry"

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

func TestIntegration(t *testing.T) {
	for _, server := range servers {
		t.Run(serverName(server), func(t *testing.T) {
			t.Run("Single", func(t *testing.T) {
				cl := &httpx.Client{
					RetryPolicy: retry.Never,
				}
				m := newMockLogger(t)
				OnClient(cl, m)
				inst := serverInstruction{StatusCode: 500, Body: []bodyChunk{
					{Data: []byte(`Green Eggs and Ham`)},
				}}
				p := inst.toPlan(parentCtx, "", httpServer)

				e, err := cl.Do(p)

				m.AssertExpectations(t)
				require.NotNil(t, e)
				require.NoError(t, err)

				seg := xray.GetSegment(e.Plan.Context())
				require.NotNil(t, seg)
				assert.False(t, seg.InProgress)
				assert.False(t, seg.Error)
				assert.True(t, seg.Fault)

				subSeg := xray.GetSegment(e.Request.Context())
				require.NotNil(t, subSeg)
				assert.Same(t, seg, subSeg.ParentSegment)
				assert.Equal(t, seg.ID, subSeg.ParentID)
				assert.Equal(t, "Attempt[0]", subSeg.Name)
				assert.Equal(t, 500, subSeg.GetHTTP().Response.Status)
				assert.False(t, seg.InProgress)
				assert.False(t, seg.Error)
				assert.True(t, seg.Fault)
			})
			t.Run("Serial Retries", func(t *testing.T) {
				cl := &httpx.Client{
					RetryPolicy: retry.NewPolicy(
						retry.Times(1).And(retry.StatusCode(429)),
						retry.DefaultWaiter,
					),
				}
				m := newMockLogger(t)
				OnClient(cl, m)
				inst := serverInstruction{StatusCode: 429, Body: []bodyChunk{
					{Data: []byte(`I so busy`)},
				}}
				p := inst.toPlan(parentCtx, "", httpServer)

				e, err := cl.Do(p)

				m.AssertExpectations(t)
				require.NotNil(t, e)
				require.NoError(t, err)

				seg := xray.GetSegment(e.Plan.Context())
				require.NotNil(t, seg)
				assert.False(t, seg.InProgress)
				assert.True(t, seg.Error)
				assert.True(t, seg.Throttle)
				assert.False(t, seg.Fault)

				subSeg := xray.GetSegment(e.Request.Context())
				require.NotNil(t, subSeg)
				assert.Same(t, seg, subSeg.ParentSegment)
				assert.Equal(t, seg.ID, subSeg.ParentID)
				assert.Equal(t, "Attempt[1]", subSeg.Name)
				assert.Equal(t, 429, subSeg.GetHTTP().Response.Status)
				assert.True(t, seg.Error)
				assert.True(t, seg.Throttle)
				assert.False(t, seg.Fault)
			})
			t.Run("Racing", func(t *testing.T) {
				cl := &httpx.Client{
					RacingPolicy: racing.NewPolicy(
						racing.NewStaticScheduler(2*time.Millisecond, 30*time.Millisecond),
						racing.AlwaysStart,
					),
				}
				m := newMockLogger(t)
				OnClient(cl, m)
				inst := serverInstruction{
					HeaderPause: 50 * time.Millisecond,
					StatusCode:  200,
					Body: []bodyChunk{
						{
							Pause: 20 * time.Millisecond,
							Data:  []byte(`I'm busy...`),
						},
						{
							Pause: 10 * time.Millisecond,
							Data:  []byte(`...but I got it done!`),
						},
					}}
				p := inst.toPlan(parentCtx, "", httpServer)

				e, err := cl.Do(p)

				m.AssertExpectations(t)
				require.NotNil(t, e)
				require.NoError(t, err)

				seg := xray.GetSegment(e.Plan.Context())
				require.NotNil(t, seg)
				assert.False(t, seg.InProgress)
				assert.False(t, seg.Error)
				assert.False(t, seg.Throttle)
				assert.False(t, seg.Fault)

				subSeg := xray.GetSegment(e.Request.Context())
				require.NotNil(t, subSeg)
				assert.Same(t, seg, subSeg.ParentSegment)
				assert.Equal(t, seg.ID, subSeg.ParentID)
				// We can't be certain which of the racing requests ended up
				// winning the race, so no point asserting on the name.
				assert.Equal(t, 200, subSeg.GetHTTP().Response.Status)
				assert.False(t, seg.Error)
				assert.False(t, seg.Throttle)
				assert.False(t, seg.Fault)
			})
		})
	}
}
