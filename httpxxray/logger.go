// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

// Logger allows the X-Ray plugin to log issues it has encountered. The
// interface is compatible with the Go standard log.Logger.
//
// Implementations of Logger must be safe for concurrent use by multiple
// goroutines.
type Logger interface {
	// Printf prints a message to the logger. Arguments are handled in
	// the manner of fmt.Printf.
	Printf(format string, v ...interface{})
}

// NopLogger implements the Logger interface but ignores all messages
// sent to it. Use NopLogger if you are not interested in learning about
// issues encountered by the X-Ray plugin.
type NopLogger struct{}

func (_ NopLogger) Printf(string, ...interface{}) {
}
