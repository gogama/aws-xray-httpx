package main

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	httpxxray "github.com/gogama/aws-xray-httpx/httpxxray/v2"
	"github.com/gogama/httpx"
	"github.com/gogama/httpx/request"
	"github.com/gogama/httpx/timeout"
	"github.com/gogama/testserv"
)

// Before running this code in your Lambda Function:
//
//    - Enable Active Tracing for your Function in the Lambda service.
//    - Use a test event which is a JSON string value.

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, evt string) (string, error) {
	// Start a test HTTPS server which will cause the following:
	//    1. On first request, return 503 Service Unavailable error (retryable)
	//    2. On second request, cause a timeout waiting for request headers.
	//    3. On third request, return a 429 Too Many Requests error (retryable)
	//    4. On fourth request, caues a timeout reading response body.
	//    5. On the fifth request, uneventfully serve a success response.
	server := httptest.NewTLSServer(&testserv.Handler{
		Inst: []testserv.Instruction{
			{StatusCode: 503},
			{StatusCode: 500, HeaderDelay: 500 * time.Millisecond},
			{StatusCode: 429, Body: []byte(`You makin' me crazy.`)},
			{StatusCode: 200, BodyDelay: 50 * time.Millisecond, BodyServiceTime: 200 * time.Millisecond, Body: []byte("I will eventually finish serving this sentence.")},
			{StatusCode: 200, Body: []byte("Success!")},
		},
	})
	defer server.Close()

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
		return "", err
	}

	e, err := cl.Do(p)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Status: %d\nBody:   %s\n", e.StatusCode(), e.Body), nil
}
