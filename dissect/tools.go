// this file prevents dependencies of tool source files from being removed
// when running `go mod tidy`.
//
// See https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
//
// go:build tools

package dissect

import _ "golang.org/x/tools/go/packages"
