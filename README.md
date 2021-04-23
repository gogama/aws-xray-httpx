httpxxray - AWS X-Ray plugin for httpx
======================================

[![Build Status](https://travis-ci.com/gogama/aws-xray-httpx.svg)](https://travis-ci.com/gogama/aws-xray-httpx) [![Go Report Card](https://goreportcard.com/badge/github.com/gogama/aws-xray-httpx/httpxxray)](https://goreportcard.com/report/github.com/gogama/aws-xray-httpx/httpxxray) [![PkgGoDev](https://pkg.go.dev/badge/github.com/gogama/aws-xray-httpx/httpxxray)](https://pkg.go.dev/github.com/gogama/aws-xray-httpx/httpxxray)

This project is an httpx plugin that publishes HTTP request traces to AWS X-Ray.

Getting Started
===============

Install httpxxray:

```sh
$ go get github.com/gogama/aws-xray-httpx/httpxxray
```

Install the plugin onto your `httpx.Client`:

```go
package main

import (
	"github.com/gogama/aws-xray-httpx/httpxxray"
	"github.com/gogama/httpx"
	"github.com/gogama/httpx/request"
)

func main() {
	// Start an X-Ray segment. (Not necessary if running within Lambda, see examples/lambda.)
	ctx, seg := xray.BeginSegment(context.Background(), "myapp")
	defer func() { seg.Close(nil) }()

	// Create an httpx client.
	client := &httpx.Client{} // Use default retry and timeout policies

	// Install the plugin.
	httpxxray.OnClient(client, httpxxray.NopLogger{})

	// Send an HTTP request and publish X-Ray trace.
	// NOTE: You must now use the context which contains the X-Ray segment!
	p, err := request.NewPlanWithContext(ctx, "GET", "http://example.com", nil)
	if err != nil {
		... // Handle error
	}
	e, err := client.Do(p) // This sends the request and publishes the plan to X-Ray.
	if err != nil {
		... // Handle error
	}
	...
}
```

For the full API reference documentation, [click here](https://pkg.go.dev/github.com/gogama/aws-xray-httpx/httpxxray).

Examples
========

The `examples/` directory contains some simple example programs:

- [`normal/`](example/normal) - Using the X-Ray plugin within a "normal" Go program.
- [`lambda/`](example/lambda) - Using the X-Ray plugin within an AWS Lambda function.
- [`racing/`](example/racing) - Using the X-Ray plugin with the httpx "racing" feature enabled.

FAQ
===

Contents:

1. [Does the plugin work with the httpx racing feature?](#1-does-the-plugin-work-with-the-httpx-racing-feature)
2. [I am getting a panic with message `failed to begin subsegment named 'example.com': segment cannot be found.`](#2-i-am-getting-a-panic-with-message-failed-to-begin-subsegment-named-examplecom-segment-cannot-be-found)

### 1. Does the plugin work with the httpx racing feature?

Yes!

It works seamlessly - just configure your httpx racing policy, as explained
[here](https://pkg.go.dev/github.com/gogama/httpx#readme-concurrent-requests-racing).

### 2. I am getting a panic with message `failed to begin subsegment named 'example.com': segment cannot be found.`

This is typically caused by one of two problems:

1. If running outside Lambda, not having a parent segment. (It is not necessary
   to create a parent segment in Lambda.)
2. Not using a context.
    - You must use `request.NewPlanWithContext` with this plugin.
    - The context must contain a valid X-Ray parent segment.

Acknowledgements
================

Developer happiness on this project was boosted by JetBrains' generous donation
of an [open source license](https://www.jetbrains.com/opensource/) for their
lovely GoLand IDE. ❤
