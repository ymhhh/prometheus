// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build ignore

package main

import (
	"log"
	"os"
	"path"
	"time"

	"github.com/shurcooL/vfsgen"

	"github.com/prometheus/prometheus/pkg/modtimevfs"
	"github.com/prometheus/prometheus/util"
	"github.com/prometheus/prometheus/web/ui"
)

func main() {
	assetsPrefix := ""
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	switch path.Base(wd) {
	case "prometheus":
		// When running Prometheus (without built-in assets) from the repo root.
		assetsPrefix = "./web/ui"
	case "web":
		// When running web tests.
		assetsPrefix = "./ui"
	case "ui":
		// When generating statically compiled-in assets.
		assetsPrefix = "./"
	}
	ui.Assets = util.SetAssets(assetsPrefix)
	fs := modtimevfs.New(ui.Assets, time.Unix(1, 0))
	err = vfsgen.Generate(fs, vfsgen.Options{
		PackageName:  "ui",
		BuildTags:    "builtinassets",
		VariableName: "Assets",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
