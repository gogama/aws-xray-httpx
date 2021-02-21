// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/aws/aws-xray-sdk-go/xray"

	"github.com/stretchr/testify/assert"

	"github.com/gogama/httpx"

	"github.com/stretchr/testify/require"

	"github.com/gogama/httpx/racing"
	"github.com/gogama/httpx/request"
)

func TestHandler_Handle(t *testing.T) {
	t.Run("unsupported event", func(t *testing.T) {
		assert.PanicsWithValue(t, "httpxxray: unsupported event", func() {
			h := &handler{&NopLogger{}}
			h.Handle(httpx.BeforeReadBody, nil)
		})
	})
	t.Run("BeforeExecutionStart[No parent segment]", func(t *testing.T) {
		e := newExecutionWithContext(t, context.TODO())
		m := newMockLogger(t)
		h := &handler{m}
		m.On("Printf", subsegmentNotStartedF, []interface{}{"BeforeExecutionStart", "foo.com"}).Once()

		h.Handle(httpx.BeforeExecutionStart, e)

		m.AssertExpectations(t)
	})
	t.Run("BeforeAttempt[No execution segment]", func(t *testing.T) {
		e := newExecutionWithContext(t, context.TODO())
		m := newMockLogger(t)
		h := &handler{m}
		m.On("Printf", subsegmentNotStartedF, []interface{}{"BeforeAttempt", "foo.com"}).Once()

		e.Request = e.Plan.ToRequest(context.TODO())
		h.Handle(httpx.BeforeAttempt, e)

		m.AssertExpectations(t)
	})
	t.Run("AfterAttempt[No attempt segment]", func(t *testing.T) {
		e := newExecutionWithContext(t, context.TODO())
		m := newMockLogger(t)
		h := &handler{m}

		e.Request = e.Plan.ToRequest(context.TODO())
		h.Handle(httpx.AfterAttempt, e)

		m.AssertExpectations(t)
	})
	t.Run("AfterAttempt[Missing execution state]", func(t *testing.T) {
		// This scenario shouldn't happen, as it implies that someone else has
		// corrupted the execution state so that the handler can find the attempt
		// subsegment, but can't find the execution state.
		e := newExecutionWithContext(t, parentCtx)
		m := newMockLogger(t)
		h := &handler{m}
		h.Handle(httpx.BeforeExecutionStart, e)
		e.Request = e.Plan.ToRequest(e.Plan.Context())
		h.Handle(httpx.BeforeAttempt, e)
		e.SetValue(executionStateKey, nil)

		assert.PanicsWithError(t, "httpxxray: no execution state", func() {
			h.Handle(httpx.AfterAttempt, e)
		})
	})
	t.Run("AfterAttempt[Missing attempt state]", func(t *testing.T) {
		// This scenario shouldn't happen, as it implies that someone else has
		// corrupted the execution state so that the handler can find the attempt
		// subsegment and the execution state, but can't find the attempt state
		// within the execution state.
		e := newExecutionWithContext(t, parentCtx)
		m := newMockLogger(t)
		h := &handler{m}
		h.Handle(httpx.BeforeExecutionStart, e)
		e.Request = e.Plan.ToRequest(e.Plan.Context())
		h.Handle(httpx.BeforeAttempt, e)
		e.SetValue(executionStateKey, &executionState{})

		assert.PanicsWithError(t, "httpxxray: no attempt state 0", func() {
			h.Handle(httpx.AfterAttempt, e)
		})
	})
	t.Run("AfterPlanTimeout[No execution segment]", func(t *testing.T) {
		e := newExecutionWithContext(t, context.TODO())
		m := newMockLogger(t)
		h := &handler{m}

		h.Handle(httpx.AfterPlanTimeout, e)

		m.AssertExpectations(t)
	})
	t.Run("AfterExecutionEnd[No execution segment]", func(t *testing.T) {
		e := newExecutionWithContext(t, context.TODO())
		m := newMockLogger(t)
		h := &handler{m}

		h.Handle(httpx.AfterExecutionEnd, e)

		m.AssertExpectations(t)
	})
	t.Run("AfterPlanTimeout[With execution segment]", func(t *testing.T) {
		ctx, seg := newNonDummySegment(t)
		defer seg.Close(nil)
		e := newExecutionWithContext(t, ctx)
		m := newMockLogger(t)
		h := &handler{m}

		h.Handle(httpx.AfterPlanTimeout, e)

		m.AssertExpectations(t)
		assert.True(t, seg.InProgress)
		require.Contains(t, seg.Metadata, "httpx")
		assert.Equal(t, true, seg.Metadata["httpx"]["plan_timeout"])
	})
	t.Run("incomplete flow", func(t *testing.T) {
		// The purpose of this test is to make sure we can close the root
		// execution segment even if there's an attempt segment that somehow
		// doesn't get closed. This could happen, for example, if an
		// AfterAttempt event handler panicked.
		e := newExecutionWithContext(t, parentCtx)
		m := newMockLogger(t)
		h := &handler{m}

		h.Handle(httpx.BeforeExecutionStart, e)
		e.Request = e.Plan.ToRequest(e.Plan.Context())
		h.Handle(httpx.BeforeAttempt, e)
		h.Handle(httpx.AfterExecutionEnd, e)

		m.AssertExpectations(t)
		executionSeg := xray.GetSegment(e.Plan.Context())
		require.NotNil(t, executionSeg)
		assert.Equal(t, "foo.com", executionSeg.Name)
		assert.False(t, executionSeg.InProgress)
	})
	t.Run("full flow", func(t *testing.T) {
		t.Run("serial[one attempt]", func(t *testing.T) {
			e := newExecutionWithContext(t, parentCtx)
			m := newMockLogger(t)
			h := &handler{m}

			h.Handle(httpx.BeforeExecutionStart, e)

			e.Request = e.Plan.ToRequest(e.Plan.Context())
			h.Handle(httpx.BeforeAttempt, e)
			executionSeg := xray.GetSegment(e.Plan.Context())
			require.NotNil(t, executionSeg)
			assert.Equal(t, "foo.com", executionSeg.Name)
			assert.Equal(t, "remote", executionSeg.Namespace)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			attemptSeg := xray.GetSegment(e.Request.Context())
			require.NotNil(t, attemptSeg)
			assert.Equal(t, "Attempt[0]", attemptSeg.Name)
			assert.Equal(t, "remote", attemptSeg.Namespace)
			assert.True(t, attemptSeg.InProgress)
			assert.Equal(t, 0.0, attemptSeg.EndTime)

			h.Handle(httpx.AfterAttempt, e)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			assert.False(t, attemptSeg.InProgress)
			assert.Greater(t, attemptSeg.EndTime, 0.0)

			h.Handle(httpx.AfterExecutionEnd, e)
			assert.False(t, executionSeg.InProgress)
			assert.Greater(t, executionSeg.EndTime, 0.0)

			m.AssertExpectations(t)
		})
		t.Run("serial[multiple attempts]", func(t *testing.T) {
			e := newExecutionWithContext(t, parentCtx)
			m := newMockLogger(t)
			h := &handler{m}

			h.Handle(httpx.BeforeExecutionStart, e)

			// Attempt 0
			e.Request = e.Plan.ToRequest(e.Plan.Context())
			h.Handle(httpx.BeforeAttempt, e)
			executionSeg := xray.GetSegment(e.Plan.Context())
			require.NotNil(t, executionSeg)
			assert.Equal(t, "foo.com", executionSeg.Name)
			assert.Equal(t, "remote", executionSeg.Namespace)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			attemptSeg := xray.GetSegment(e.Request.Context())
			require.NotNil(t, attemptSeg)
			assert.Equal(t, "Attempt[0]", attemptSeg.Name)
			assert.Equal(t, "remote", attemptSeg.Namespace)
			assert.True(t, attemptSeg.InProgress)
			assert.Equal(t, 0.0, attemptSeg.EndTime)

			// Attempt 1
			e.Request = e.Plan.ToRequest(e.Plan.Context())
			e.Attempt = 1
			h.Handle(httpx.BeforeAttempt, e)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			attemptSeg = xray.GetSegment(e.Request.Context())
			require.NotNil(t, attemptSeg)
			assert.Equal(t, "Attempt[1]", attemptSeg.Name)
			assert.True(t, attemptSeg.InProgress)
			assert.Equal(t, 0.0, attemptSeg.EndTime)

			h.Handle(httpx.AfterAttempt, e)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			assert.False(t, attemptSeg.InProgress)
			assert.Greater(t, attemptSeg.EndTime, 0.0)

			h.Handle(httpx.AfterExecutionEnd, e)
			assert.False(t, executionSeg.InProgress)
			assert.Greater(t, executionSeg.EndTime, 0.0)

			m.AssertExpectations(t)
		})
		t.Run("racing[multiple attempts]", func(t *testing.T) {
			e := newExecutionWithContext(t, parentCtx)
			m := newMockLogger(t)
			h := &handler{m}

			// EXECUTION: START
			h.Handle(httpx.BeforeExecutionStart, e)

			// Attempt 0: START
			req0 := e.Plan.ToRequest(e.Plan.Context())
			e.Request = req0
			e.Attempt = 0
			h.Handle(httpx.BeforeAttempt, e)
			req0 = e.Request
			executionSeg := xray.GetSegment(e.Plan.Context())
			require.NotNil(t, executionSeg)
			assert.Equal(t, "foo.com", executionSeg.Name)
			assert.Equal(t, "remote", executionSeg.Namespace)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			attempt0Seg := xray.GetSegment(req0.Context())
			require.NotNil(t, attempt0Seg)
			assert.Equal(t, "Attempt[0]", attempt0Seg.Name)
			assert.Equal(t, "remote", attempt0Seg.Namespace)
			assert.True(t, attempt0Seg.InProgress)
			assert.Equal(t, 0.0, attempt0Seg.EndTime)

			// Attempt 1: START
			req1 := e.Plan.ToRequest(e.Plan.Context())
			e.Request = req1
			e.Attempt = 1
			h.Handle(httpx.BeforeAttempt, e)
			req1 = e.Request
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			attempt1Seg := xray.GetSegment(req1.Context())
			require.NotNil(t, attempt1Seg)
			assert.Equal(t, "Attempt[1]", attempt1Seg.Name)
			assert.Equal(t, "remote", attempt1Seg.Namespace)
			assert.True(t, attempt0Seg.InProgress)
			assert.Equal(t, 0.0, attempt0Seg.EndTime)
			assert.True(t, attempt1Seg.InProgress)
			assert.Equal(t, 0.0, attempt1Seg.EndTime)

			// Attempt 1: END
			e.Request = req1
			e.Response = &http.Response{
				StatusCode: 400,
			}
			e.Attempt = 1
			h.Handle(httpx.AfterAttempt, e)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			assert.False(t, attempt1Seg.InProgress)
			assert.Greater(t, attempt1Seg.EndTime, 0.0)
			assert.Equal(t, 400, attempt1Seg.GetHTTP().GetResponse().Status)
			assert.True(t, attempt1Seg.Error)
			assert.False(t, attempt1Seg.Fault)
			assert.True(t, attempt0Seg.InProgress)
			assert.Equal(t, 0.0, attempt0Seg.EndTime)

			// Attempt 0: END
			e.Request = req0
			e.Response = nil
			e.Err = racing.Redundant
			e.Attempt = 0
			h.Handle(httpx.AfterAttempt, e)
			assert.True(t, executionSeg.InProgress)
			assert.Equal(t, 0.0, executionSeg.EndTime)
			assert.False(t, attempt0Seg.InProgress)
			assert.Greater(t, attempt0Seg.EndTime, 0.0)
			assert.GreaterOrEqual(t, attempt0Seg.EndTime, attempt1Seg.EndTime)
			assert.Equal(t, 0, attempt0Seg.GetHTTP().GetResponse().Status)
			assert.False(t, attempt0Seg.Error)
			assert.True(t, attempt0Seg.Fault)

			// Execution: END
			e.Err = nil
			h.Handle(httpx.AfterExecutionEnd, e)
			assert.False(t, executionSeg.InProgress)
			assert.Greater(t, executionSeg.EndTime, 0.0)
			assert.False(t, executionSeg.Error)
			assert.False(t, executionSeg.Fault)

			m.AssertExpectations(t)
		})
	})
}

func newExecutionWithContext(t *testing.T, ctx context.Context) *request.Execution {
	p, err := request.NewPlanWithContext(ctx, "", "http://foo.com", nil)
	require.NotNil(t, p)
	require.NoError(t, err)
	return &request.Execution{
		Plan: p,
	}
}

func TestHost(t *testing.T) {
	p := &request.Plan{}
	p.Host = "foo"
	var err error
	p.URL, err = url.Parse("http://bar.com")
	require.NoError(t, err)
	assert.Equal(t, "foo", host(p))
	p.Host = ""
	assert.Equal(t, "bar.com", host(p))
	p.Host = "baz.com"
	p.URL = nil
	assert.Equal(t, "baz.com", host(p))
}

func TestSetSegmentHTTPResponse(t *testing.T) {
	t.Run("No response", func(t *testing.T) {
		_, seg := newNonDummySegment(t)
		defer seg.Close(nil)

		setSegmentHTTPResponse(seg, nil)

		assert.False(t, seg.Error)
		assert.False(t, seg.Fault)
		assert.False(t, seg.Throttle)
	})
	t.Run("OK", func(t *testing.T) {
		_, seg := newNonDummySegment(t)
		defer seg.Close(nil)

		setSegmentHTTPResponse(seg, &http.Response{StatusCode: 200})

		assert.False(t, seg.Error)
		assert.False(t, seg.Fault)
		assert.False(t, seg.Throttle)
	})
	t.Run("Error.4XX", func(t *testing.T) {
		statusCodes := []int{400, 401, 403, 404, 405, 406, 409}
		for _, statusCode := range statusCodes {
			t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
				_, seg := newNonDummySegment(t)
				defer seg.Close(nil)

				setSegmentHTTPResponse(seg, &http.Response{StatusCode: statusCode})

				assert.True(t, seg.Error)
				assert.False(t, seg.Fault)
				assert.False(t, seg.Throttle)
			})
		}
	})
	t.Run("Error.429", func(t *testing.T) {
		_, seg := newNonDummySegment(t)
		defer seg.Close(nil)

		setSegmentHTTPResponse(seg, &http.Response{StatusCode: 429})

		assert.True(t, seg.Error)
		assert.False(t, seg.Fault)
		assert.True(t, seg.Throttle)
	})
	t.Run("Fault.5XX", func(t *testing.T) {
		statusCodes := []int{500, 502, 503, 504, 505}
		for _, statusCode := range statusCodes {
			t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
				_, seg := newNonDummySegment(t)
				defer seg.Close(nil)

				setSegmentHTTPResponse(seg, &http.Response{StatusCode: statusCode})

				assert.False(t, seg.Error)
				assert.True(t, seg.Fault)
				assert.False(t, seg.Throttle)
			})
		}
	})
}

func TestSetSegmentBodyLen(t *testing.T) {
	t.Run("No body", func(t *testing.T) {
		_, seg := newNonDummySegment(t)
		defer seg.Close(nil)

		setSegmentBodyLen(seg, nil)

		assert.NotContains(t, "httpx", seg.Metadata)
	})
	t.Run("Empty body", func(t *testing.T) {
		_, seg := newNonDummySegment(t)
		defer seg.Close(nil)

		setSegmentBodyLen(seg, []byte{})

		require.Contains(t, seg.Metadata, "httpx")
		require.Contains(t, seg.Metadata["httpx"], "body_length")
		assert.Equal(t, 0, seg.Metadata["httpx"]["body_length"])
	})
	t.Run("Non-empty body", func(t *testing.T) {
		_, seg := newNonDummySegment(t)
		defer seg.Close(nil)

		setSegmentBodyLen(seg, []byte("foo"))

		require.Contains(t, seg.Metadata, "httpx")
		require.Contains(t, seg.Metadata["httpx"], "body_length")
		assert.Equal(t, 3, seg.Metadata["httpx"]["body_length"])
	})
}

func TestSetSegmentExecutionMetadata(t *testing.T) {
	_, seg := newNonDummySegment(t)
	defer seg.Close(nil)

	setSegmentExecutionMetadata(seg, 31, 33)

	require.Contains(t, seg.Metadata, "httpx")
	require.Contains(t, seg.Metadata["httpx"], "attempts")
	assert.Equal(t, 31, seg.Metadata["httpx"]["attempts"])
	require.Contains(t, seg.Metadata["httpx"], "waves")
	assert.Equal(t, 33, seg.Metadata["httpx"]["waves"])
}

func TestSetSegmentAttemptMetadata(t *testing.T) {
	_, seg := newNonDummySegment(t)
	defer seg.Close(nil)

	setSegmentAttemptMetadata(seg, 109)

	require.Contains(t, seg.Metadata, "httpx")
	require.Contains(t, seg.Metadata["httpx"], "attempt")
	assert.Equal(t, 109, seg.Metadata["httpx"]["attempt"])
}

func TestGetAttemptState(t *testing.T) {
	t.Run("No execution state", func(t *testing.T) {
		as, err := getAttemptState(&request.Execution{})

		assert.Equal(t, attemptState{}, as)
		assert.EqualError(t, err, "httpxxray: no execution state")
	})
	t.Run("No attempt state", func(t *testing.T) {
		e := &request.Execution{}
		e.SetValue(executionStateKey, &executionState{})
		as, err := getAttemptState(e)

		assert.Equal(t, attemptState{}, as)
		assert.EqualError(t, err, "httpxxray: no attempt state 0")
	})
}

func TestPutAttemptState(t *testing.T) {
	t.Run("No attempt skip", func(t *testing.T) {
		e := &request.Execution{}

		httpSubsegments := &xray.HTTPSubsegments{}
		putAttemptState(e, attemptState{httpSubsegments: httpSubsegments})
		as, err := getAttemptState(e)

		require.NoError(t, err)
		assert.Same(t, httpSubsegments, as.httpSubsegments)
	})
	t.Run("With attempt skip", func(t *testing.T) {
		e := &request.Execution{Attempt: 1}

		httpSubsegments := &xray.HTTPSubsegments{}
		putAttemptState(e, attemptState{httpSubsegments: httpSubsegments})
		e.Attempt = 0
		as0, err0 := getAttemptState(e)
		e.Attempt = 1
		as1, err1 := getAttemptState(e)

		require.NoError(t, err0)
		assert.Nil(t, as0.httpSubsegments)
		require.NoError(t, err1)
		assert.Same(t, httpSubsegments, as1.httpSubsegments)
	})
	t.Run("Modify value", func(t *testing.T) {
		e := &request.Execution{}

		httpSubsegmentsBefore := &xray.HTTPSubsegments{}
		httpSubsegmentsAfter := &xray.HTTPSubsegments{}
		putAttemptState(e, attemptState{httpSubsegments: httpSubsegmentsBefore})
		asBefore, errBefore := getAttemptState(e)
		putAttemptState(e, attemptState{httpSubsegments: httpSubsegmentsAfter})
		asAfter, errAfter := getAttemptState(e)

		require.NoError(t, errBefore)
		assert.Same(t, httpSubsegmentsBefore, asBefore.httpSubsegments)
		require.NoError(t, errAfter)
		assert.Same(t, httpSubsegmentsAfter, asAfter.httpSubsegments)
	})
}

func newNonDummySegment(t *testing.T) (context.Context, *xray.Segment) {
	ctx, seg := xray.BeginSubsegment(parentCtx, "test")
	require.NotNil(t, ctx)
	require.NotNil(t, seg)
	seg.Lock()
	defer seg.Unlock()
	seg.Dummy = false // Ensure metadata can be stored on the segment
	return ctx, seg
}
