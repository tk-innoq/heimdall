// Copyright 2023 Dimitrij Drus <dadrus@gmx.de>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"net/url"

	"github.com/goccy/go-json"
)

type Backend struct {
	Host        string       `json:"host"    yaml:"host"`
	URLRewriter *URLRewriter `json:"rewrite" yaml:"rewrite"`
}

func (f *Backend) CreateURL(value *url.URL) *url.URL {
	upstreamURL := &url.URL{
		Scheme:   value.Scheme,
		Host:     f.Host,
		Path:     value.Path,
		RawQuery: value.RawQuery,
	}

	if f.URLRewriter != nil {
		f.URLRewriter.Rewrite(upstreamURL)
	}

	return upstreamURL
}

func (f *Backend) DeepCopyInto(out *Backend) {
	if f == nil {
		return
	}

	jsonStr, _ := json.Marshal(f)

	// we cannot do anything with an error here as
	// the interface implemented here doesn't support
	// error responses
	json.Unmarshal(jsonStr, out) //nolint:errcheck
}
