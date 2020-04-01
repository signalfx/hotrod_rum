// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.

package main

import (
	"github.com/signalfx/hotrod_rum/cmd"
)

//go:generate esc -pkg frontend -o services/frontend/gen_assets.go -prefix services/frontend/web_assets services/frontend/web_assets
func main() {
	cmd.Execute()
}
