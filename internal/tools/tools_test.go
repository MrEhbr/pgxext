//go:build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint" // required by rules.mk
	_ "mvdan.cc/gofumpt"                                    // required by rules.mk
)
