// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-xray-sdk-go/strategy/ctxmissing"

	"github.com/aws/aws-xray-sdk-go/xray"
)

func TestMain(m *testing.M) {
	// Configure X-Ray SDK to not panic on missing context. For us there's
	// no point letting it panic because it just means we need to recover
	// from the panic in test scenarios we've deliberately set up to be
	// missing an X-Ray parent segment.
	err := xray.Configure(xray.Config{
		ContextMissingStrategy: &ctxmissing.DefaultIgnoreErrorStrategy{},
	})
	if err != nil {
		panic("failed to configure X-Ray")
	}

	// Start test servers.
	httpServer.Start()
	defer httpServer.Close()
	httpsServer.StartTLS()
	defer httpsServer.Close()
	http2Server.EnableHTTP2 = true
	http2Server.Start()
	defer http2Server.Close()
	waitForServerStart(httpServer)
	waitForServerStart(httpsServer)
	waitForServerStart(http2Server)

	// Run tests.
	os.Exit(m.Run())
}

// Use this context in tests that want to simulate a parent context which does
// contain an X-Ray segment.
//
// The context is based around the way in which the Go runtime for an AWS Lambda
// function communicates the function's X-Ray trace ID to the AWS X-Ray SDK for
// Go.
//
// References:
//
// - https://github.com/aws/aws-lambda-go/blob/master/lambda/function.go
// - https://github.com/aws/aws-xray-sdk-go/blob/master/xray/lambda.go
var parentCtx = context.WithValue(context.Background(), "x-amzn-trace-id", "simulated Lambda X-Ray trace ID")
