// Copyright 2017 Delving B.V.
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

package handlers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	c "github.com/delving/hub3/config"

	"github.com/delving/hub3/hub3/index"
	"github.com/go-chi/chi"
	"github.com/go-chi/render"
)

func RegisterElasticSearchProxy(router chi.Router) {
	r := chi.NewRouter()

	r.Get("/indexes", func(w http.ResponseWriter, r *http.Request) {
		indexes, err := index.ListIndexes()
		if err != nil {
			log.Print(err)
		}
		render.PlainText(w, r, fmt.Sprint("indexes:", indexes))
	})

	if c.Config.ElasticSearch.Proxy {
		r.HandleFunc("/*", esProxy)
	}

	router.Mount("/api/es", r)
}

func esProxy(w http.ResponseWriter, r *http.Request) {
	// parse the url
	esURL, _ := url.Parse(c.Config.ElasticSearch.Urls[0])

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(esURL)

	// strip prefix from path
	r.URL.Path = strings.TrimPrefix(r.URL.EscapedPath(), "/api/es")

	switch {
	case strings.HasSuffix(r.URL.EscapedPath(), "/_analyze") && r.Method == "POST":
		// allow post requests on analyze
	case r.Method != "GET":
		http.Error(w, fmt.Sprintf("method %s is not allowed on esProxy", r.Method), http.StatusBadRequest)
		return
	case r.URL.Path == "/":
		// root is allowed to provide version
	case strings.HasPrefix(r.URL.EscapedPath(), fmt.Sprintf("/%s", c.Config.ElasticSearch.GetIndexName())):
		// direct access on get is allowed via the proxy
	case !strings.HasPrefix(r.URL.EscapedPath(), "/_cat"):
		http.Error(w, fmt.Sprintf("path %s is not allowed on esProxy", r.URL.EscapedPath()), http.StatusBadRequest)
		return
	}

	// Update the headers to allow for SSL redirection
	r.URL.Host = esURL.Host
	r.URL.Scheme = esURL.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = esURL.Host

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(w, r)
}
