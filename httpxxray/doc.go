// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

/*
Package httpxxray adds AWS X-Ray support to the httpx library's robust
HTTP client. See https://github.com/gogama/httpx.

Use the OnClient function to install X-Ray support in any httpx.Client:

	cl := &httpx.Client{}               // Create robust HTTP client
	httpxxray.OnClient(cl, nil)         // Install X-Ray plugin

When creating a request plan for the client to execute, use an X-Ray
aware context, for example the aws.Context passed to a Lambda function
handler.

	pl := request.NewPlanWithContext(   // Make plan using X-Ray aware context
		xrayAwareContext,
		"GET",
		"https://www.example.com/things/123",
		nil,
	)
	e, err := cl.Do(pl)                 // Send request and read response

	// If the context is sampled by X-Ray, the X-Ray trace for the HTTP
	// request has now been emitted.

Use the OnHandlers function to install X-Ray support directly onto an
httpx.HandlerGroup.
*/
package httpxxray
