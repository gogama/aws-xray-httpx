// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gogama/httpx"
	"github.com/gogama/httpx/request"
)

type handler struct {
	logger Logger
}

func (h *handler) Handle(evt httpx.Event, e *request.Execution) {
	switch evt {
	case httpx.BeforeExecutionStart:
		beforeExecutionStart(h.logger, e)
	case httpx.BeforeAttempt:
		beforeAttempt(h.logger, e)
	case httpx.AfterAttempt:
		afterAttempt(e)
	case httpx.AfterPlanTimeout:
		afterPlanTimeout(e)
	case httpx.AfterExecutionEnd:
		afterExecutionEnd(e)
	default:
		panic("httpxxray: unsupported event")
	}
}

func beforeExecutionStart(l Logger, e *request.Execution) {
	ctx, seg := xray.BeginSubsegment(e.Plan.Context(), host(e.Plan))
	if seg == nil {
		logSubsegmentNotStarted(httpx.BeforeExecutionStart, l, e.Plan)
		return
	}

	seg.Lock()
	defer seg.Unlock()
	seg.Namespace = "remote"

	e.Plan = e.Plan.WithContext(ctx)
}

func afterExecutionEnd(e *request.Execution) {
	seg := xray.GetSegment(e.Plan.Context())
	if seg == nil {
		return
	}
	defer seg.Close(e.Err)
	setSegmentHTTPResponse(seg, e.Response)
	setSegmentBodyLen(seg, e.Body)
	setSegmentExecutionMetadata(seg, e.Attempt+1, e.Wave+1)
}

func beforeAttempt(l Logger, e *request.Execution) {
	ctx, seg := xray.BeginSubsegment(e.Request.Context(), fmt.Sprintf("Attempt:%d", e.Attempt))
	if seg == nil {
		logSubsegmentNotStarted(httpx.BeforeAttempt, l, e.Plan)
		return
	}

	setSegmentAttemptMetadata(seg, e.Attempt)

	httpSubsegments, trace := newClientTrace(ctx)
	ctx = httptrace.WithClientTrace(ctx, trace)
	req := e.Request.WithContext(ctx)

	seg.Lock()
	defer seg.Unlock()
	reqData := seg.GetHTTP().GetRequest()
	reqData.Method = req.Method
	reqData.URL = stripQuery(*req.URL)
	req.Header.Set(xray.TraceIDHeaderKey, seg.DownstreamHeader().String())

	putAttemptState(e, attemptState{httpSubsegments: httpSubsegments})
	e.Request = req
}

func afterAttempt(e *request.Execution) {
	ctx := e.Request.Context()
	seg := xray.GetSegment(ctx)
	if seg == nil {
		return
	}

	defer seg.Close(e.Err)

	as, err := getAttemptState(e)
	if err != nil {
		panic(err)
	}

	setSegmentHTTPResponse(seg, e.Response)
	setSegmentBodyLen(seg, e.Body)

	// Emulate GotConn call within Capture closure in X-Ray SDK: xray/client.go.
	if e.Err != nil {
		as.httpSubsegments.GotConn(nil, e.Err)
	}
}

func afterPlanTimeout(e *request.Execution) {
	ctx := e.Plan.Context()
	seg := xray.GetSegment(ctx)
	if seg == nil {
		return
	}
	_ = seg.AddMetadataToNamespace("httpx", "plan_timeout", true)
}

func host(p *request.Plan) string {
	if p.Host != "" {
		return p.Host
	}

	return p.URL.Host
}

func newClientTrace(ctx context.Context) (*xray.HTTPSubsegments, *httptrace.ClientTrace) {
	httpSubsegments := xray.NewHTTPSubsegments(ctx)
	return httpSubsegments, &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			httpSubsegments.GetConn(hostPort)
		},
		DNSStart: func(info httptrace.DNSStartInfo) {
			httpSubsegments.DNSStart(info)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			httpSubsegments.DNSDone(info)
		},
		ConnectStart: func(network, addr string) {
			httpSubsegments.ConnectStart(network, addr)
		},
		ConnectDone: func(network, addr string, err error) {
			httpSubsegments.ConnectDone(network, addr, err)
		},
		TLSHandshakeStart: func() {
			httpSubsegments.TLSHandshakeStart()
		},
		TLSHandshakeDone: func(connState tls.ConnectionState, err error) {
			httpSubsegments.TLSHandshakeDone(connState, err)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			httpSubsegments.GotConn(&info, nil)
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			httpSubsegments.WroteRequest(info)
		},
		GotFirstResponseByte: func() {
			httpSubsegments.GotFirstResponseByte()
		},
	}
}

func stripQuery(u url.URL) string {
	u.RawQuery = ""
	return u.String()
}

func setSegmentHTTPResponse(seg *xray.Segment, resp *http.Response) {
	if resp == nil {
		return
	}

	seg.Lock()
	defer seg.Unlock()

	// Emulate HTTP header handling logic within Capture closure in X-Ray
	// SDK: xray/client.go.
	respData := seg.GetHTTP().GetResponse()
	respData.Status = resp.StatusCode
	respData.ContentLength, _ = strconv.Atoi(resp.Header.Get("Content-Length"))
	switch resp.StatusCode / 100 {
	case 4:
		seg.Error = true
		if resp.StatusCode == 429 {
			seg.Throttle = true
		}
	case 5:
		seg.Fault = true
	}
}

func setSegmentBodyLen(seg *xray.Segment, body []byte) {
	// Add body length if available. A nil body means the request attempt
	// errored out before the response body could be read, whereas a non-
	// nil zero-length body means the response body was successfully read
	// but empty.
	if body != nil {
		_ = seg.AddMetadataToNamespace("httpx", "body_length", len(body))
	}
}

func setSegmentExecutionMetadata(seg *xray.Segment, attempts int, waves int) {
	_ = seg.AddMetadataToNamespace("httpx", "attempts", attempts)
	_ = seg.AddMetadataToNamespace("httpx", "waves", waves)
}

func setSegmentAttemptMetadata(seg *xray.Segment, attempt int) {
	_ = seg.AddMetadataToNamespace("httpx", "attempt", attempt)
}

type executionStateKeyType int

var executionStateKey = new(executionStateKeyType)

type executionState struct {
	as []attemptState
}

type attemptState struct {
	httpSubsegments *xray.HTTPSubsegments
}

func putAttemptState(e *request.Execution, as attemptState) {
	es, _ := e.Value(executionStateKey).(*executionState)
	if es == nil {
		es = &executionState{}
		e.SetValue(executionStateKey, es)
	}
	if len(es.as) == e.Attempt {
		es.as = append(es.as, attemptState{})
	} else if len(es.as) < e.Attempt {
		tmp := make([]attemptState, e.Attempt+1)
		copy(tmp, es.as)
		es.as = tmp
	}
	es.as[e.Attempt] = as
}

func getAttemptState(e *request.Execution) (attemptState, error) {
	es, _ := e.Value(executionStateKey).(*executionState)
	if es == nil {
		return attemptState{}, errors.New("httpxxray: no execution state")
	}
	if len(es.as) <= e.Attempt {
		return attemptState{}, fmt.Errorf("httpxxray: no attempt state %d", e.Attempt)
	}
	return es.as[e.Attempt], nil
}

const subsegmentNotStartedF = "httpxxray: [WARN] Unable to begin X-Ray subsegment in event %s (%s)"

func logSubsegmentNotStarted(evt httpx.Event, l Logger, p *request.Plan) {
	l.Printf(subsegmentNotStartedF, evt.Name(), host(p))
}
