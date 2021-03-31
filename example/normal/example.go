package main

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gogama/aws-xray-httpx/httpxxray"
	"github.com/gogama/httpx"
	"github.com/gogama/httpx/request"
	"github.com/gogama/httpx/timeout"
	"github.com/gogama/testserv"
)

// Before running the program, make sure you have the X-Ray daemon installed and
// running (see AWS X-Ray documentation for more detail).
//
//    e.g.
//
//      $ xray -o -n us-west-2 -f /tmp/xray.log &

func main() {
	// Start a test HTTPS server which will cause the following:
	//    1. On first request, return 503 Service Unavailable error (retryable)
	//    2. On second request, cause a timeout waiting for request headers.
	//    3. On third request, return a 429 Too Many Requests error (retryable)
	//    4. On fourth request, caues a timeout reading response body.
	//    5. On the fifth request, uneventfully serve a success response.
	server := httptest.NewTLSServer(&testserv.Handler{
		Inst: []testserv.Instruction{
			{StatusCode: 503},
			{StatusCode: 500, HeaderDelay: 5 * time.Second},
			{StatusCode: 429, Body: []byte(`You makin' me crazy.`)},
			{StatusCode: 200, BodyDelay: 50 * time.Millisecond, BodyServiceTime: 5 * time.Second, Body: []byte("I will eventually finish serving this sentence.")},
			{StatusCode: 200, Body: []byte("Success!")},
		},
	})
	defer server.Close()

	// Start an X-Ray segment for the example application.
	ctx, seg := xray.BeginSegment(context.Background(), "example/normal")
	defer func() {
		seg.Close(nil)
	}()

	// Create the robust httpx.Client and install the X-Ray plugin.
	cl := &httpx.Client{
		HTTPDoer:      server.Client(),
		TimeoutPolicy: timeout.Fixed(100 * time.Millisecond),
	}
	logger := log.New(os.Stdout, "httpxxray", log.Ldate|log.Ltime)
	httpxxray.OnClient(cl, logger)

	// Use the robust httpx.Client to send a request, and print the response.
	p, err := request.NewPlanWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		fail(seg, err)
	}

	e, err := cl.Do(p)
	if err != nil {
		fail(seg, err)
	}

	fmt.Printf("Status: %d\nBody:   %s\n", e.StatusCode(), e.Body)
}

func fail(seg *xray.Segment, err error) {
	seg.Close(err)
	println(err.Error())
	os.Exit(1)
}
