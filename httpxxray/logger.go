// Copyright 2021 The httpxxray Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package httpxxray

type Logger interface {
	Printf(format string, v ...interface{})
}

type NopLogger struct{}

func (_ NopLogger) Printf(string, ...interface{}) {
}
