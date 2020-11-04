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

// nolint:gocritic
package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/delving/hub3/ikuzo/domain"
	"github.com/matryer/is"
)

func TestMemoryStore(t *testing.T) {
	is := is.New(t)
	ctx := context.TODO()

	store := NewOrganizationStore()
	orgs, err := store.Filter(ctx)
	is.NoErr(err)
	is.Equal(len(orgs), 0)

	// test put
	orgID, err := domain.NewOrganizationID("demo")
	is.NoErr(err)

	org := &domain.Organization{
		ID: orgID,
		Config: domain.OrganizationConfig{
			Domains: []string{"localhost", "demo.localhost"},
		},
	}

	err = store.Put(ctx, org)
	is.NoErr(err)
	is.Equal(len(store.organizations), 1)
	is.Equal(len(store.domains), len(org.Config.Domains))

	// should have one org
	orgs, err = store.Filter(ctx)
	is.NoErr(err)
	is.Equal(len(orgs), len(store.organizations))

	// filter unknown domain
	orgs, err = store.Filter(ctx, domain.OrganizationFilter{Domain: "unknown.localhost"})
	is.True(errors.Is(err, domain.ErrOrgNotFound))
	is.Equal(len(orgs), 0)

	// filter known domain
	orgs, err = store.Filter(ctx, domain.OrganizationFilter{Domain: "demo.localhost"})
	is.NoErr(err)
	is.Equal(len(orgs), 1)

	// get an org
	getOrgID, err := store.Get(ctx, orgID)
	is.NoErr(err)
	is.Equal(orgID, getOrgID.ID)

	// delete an org
	err = store.Delete(ctx, orgID)
	is.NoErr(err)
	orgs, err = store.Filter(ctx)
	is.NoErr(err)
	is.Equal(len(orgs), 0)

	// org not found
	_, err = store.Get(ctx, orgID)
	is.True(errors.Is(err, domain.ErrOrgNotFound))
}

func TestService_Shutdown(t *testing.T) {
	is := is.New(t)

	ts := NewOrganizationStore()

	is.True(!ts.shutdownCalled)

	err := ts.Shutdown(context.TODO())
	is.NoErr(err)

	is.True(ts.shutdownCalled)
}
