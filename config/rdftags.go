// Copyright © 2017 Delving B.V. <info@delving.eu>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import "sync"

// RDFTag holds tag information how to tag predicate values
type RDFTag struct {
	Label     []string `json:"label"`
	Thumbnail []string `json:"thumbnail"`
	LatLong   []string `json:"latLong"`
	Date      []string `json:"date"`
	DateRange []string `json:"dateRange"`
}

// RDFTagMap contains all the namespaces
type RDFTagMap struct {
	sync.RWMutex
	tag2uri  map[string]string
	uri2tags map[string][]string
}
