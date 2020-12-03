// Copyright 2020 Delving B.V.
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

package middleware

import (
	"net/http"
	"net/url"
	"time"

	"github.com/delving/hub3/ikuzo/domain"
	"github.com/go-chi/chi"
	"github.com/justinas/alice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

// RequestLogger creates a middleware chain for request logging
func RequestLogger(log *zerolog.Logger) func(next http.Handler) http.Handler {
	c := alice.New()

	// Install the logger handler with default output on the console
	c = c.Append(hlog.NewHandler(*log))

	// Install some provided extra handler to set some request's context fields.
	// Thanks to those handler, all our logs will come with some pre-populated fields.
	c = c.Append(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		l := hlog.FromRequest(r).Info().
			// Str("orgID", organization.GetOrganizationID(r)).
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Dict("params", LogParamsAsDict(r.URL.Query()))

		setChiURLParams(l, r, "spec", "dataset_id")
		setChiURLParams(l, r, "datasetID", "dataset_id")
		setChiURLParams(l, r, "inventoryID", "inventory_id")

		l.Msg("")
	}))
	c = c.Append(hlog.RemoteAddrHandler("ip"))
	c = c.Append(hlog.UserAgentHandler("user_agent"))
	c = c.Append(hlog.RefererHandler("referer"))
	c = c.Append(hlog.RequestIDHandler("req_id", "Request-Id"))
	c = c.Append(orgIDHandler("org_id"))

	// Here is your final handler
	return c.Then
}

func setChiURLParams(l *zerolog.Event, r *http.Request, paramKey, fieldKey string) {
	if val := chi.URLParamFromCtx(r.Context(), paramKey); val != "" {
		l.Str(fieldKey, val)
	}
}

// orgIDHandler adds the request's domain.OrganizationID as a field to the context's logger
// using fieldKey as field key.
func orgIDHandler(fieldKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if orgID := domain.GetOrganizationID(r); orgID != "" {
				l := zerolog.Ctx(r.Context())
				l.UpdateContext(func(c zerolog.Context) zerolog.Context {
					return c.Str(fieldKey, string(orgID))
				})
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LogParamsAsDict logs the request params as a zerolog.Dict.
func LogParamsAsDict(params url.Values) *zerolog.Event {
	dict := zerolog.Dict()

	for key, values := range params {
		arr := zerolog.Arr()

		var nonEmpty bool

		for _, v := range values {
			if v != "" {
				arr = arr.Str(v)

				if !nonEmpty {
					nonEmpty = true
				}
			}
		}

		if nonEmpty {
			dict = dict.Array(key, arr)
		}
	}

	return dict
}
