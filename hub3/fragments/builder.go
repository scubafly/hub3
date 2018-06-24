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

package fragments

import (
	"bytes"
	fmt "fmt"
	"io"
	"log"
	"net/url"
	"strings"

	c "github.com/delving/rapid-saas/config"
	r "github.com/kiivihal/rdf2go"
)

// FragmentBuilder holds all the information to build and store Fragments
type FragmentBuilder struct {
	fg             *FragmentGraph
	Graph          *r.Graph
	ResourceLabels map[string]string
	resources      *ResourceMap
}

// ResourcesList returns a list of FragmentResource
func (rm *ResourceMap) ResourcesList() []*FragmentResource {
	rs := []*FragmentResource{}
	for _, entry := range rm.resources {
		err := entry.SetEntries(rm)
		if err != nil {
			log.Printf("Unable to set entries: %s", err)
		}
		rs = append(rs, entry)
	}
	return rs
}

// ResourceMap returns a *ResourceMap for the Graph in the FragmentBuilder
func (fb *FragmentBuilder) ResourceMap() (*ResourceMap, error) {
	if fb.resources == nil {
		rm, err := NewResourceMap(fb.Graph)
		if err != nil {
			log.Printf("unable to create resourceMap due to %s", err)
			return nil, err
		}
		fb.resources = rm
	}
	return fb.resources, nil
}

// NewFragmentBuilder creates a new instance of the FragmentBuilder
func NewFragmentBuilder(fg *FragmentGraph) *FragmentBuilder {
	return &FragmentBuilder{
		fg:             fg,
		Graph:          r.NewGraph(""),
		ResourceLabels: map[string]string{},
	}
}

// NewFragmentGraph creates a new instance of FragmentGraph
func NewFragmentGraph() *FragmentGraph {
	return &FragmentGraph{
		Meta: &Header{
			DocType: FragmentGraphDocType,
		},
	}
}

// FragmentGraph gives access to the FragmentGraph object from the Builder struct
func (fb *FragmentBuilder) FragmentGraph() *FragmentGraph {
	return fb.fg
}

// CreateFragment creates a fragment from a triple
//func (fb *FragmentBuilder) CreateFragment(triple *r.Triple) (*Fragment, error) {
//f := &Fragment{}
//f.Subject = triple.Subject.RawValue()
//f.Predicate = triple.Predicate.RawValue()
//label, _ := c.Config.NameSpaceMap.GetSearchLabel(f.GetPredicate())
//f.SearchLabel = label
//f.Object = triple.Object.RawValue()
//f.Triple = triple.String()

//switch triple.Object.(type) {
//case *r.Literal:
//f.ObjectType = ObjectType_LITERAL
//f.ObjectTypeRaw = "literal"
//l := triple.Object.(*r.Literal)
//f.Language = l.Language
//// Set default datatypes
//f.DataType = ObjectXSDType_STRING
//f.XSDRaw, _ = f.GetDataType().GetPrefixLabel()
//if l.Datatype != nil {
//xsdType, err := GetObjectXSDType(l.Datatype.String())
//if err != nil {
//log.Printf("Unable to get xsdType for %s", l.Datatype.String())
//break
//}
//prefixLabel, err := xsdType.GetPrefixLabel()
//if err != nil {
//log.Printf(
//"Unable to get xsdType prefix label for %s (%s)",
//l.Datatype.String(),
//xsdType.String(),
//)
//break
//}
//f.XSDRaw = prefixLabel
//f.DataType = xsdType
//}
//case *r.Resource, *r.BlankNode:
//f.ObjectType = ObjectType_RESOURCE
//f.ObjectTypeRaw = "resource"
//if f.IsTypeLink() {
//f.AddTags("typelink")
//}
////f.TypeLink = f.IsTypeLink()
////if fg.Graph.Len() == 0 {
////log.Printf("Warn: Graph is empty can't do linking checks\n")
////break
////}
////f.GraphExternalLink = fg.IsGraphExternal(triple.Object)
////isDomainExternal, err := fg.IsDomainExternal(f.Object)
////if err != nil {
////log.Printf("Unable to parse object domain: %#v", err)
////break
////}
////f.DomainExternalLink = isDomainExternal
//default:
//return f, fmt.Errorf("unknown object type: %#v", triple.Object)
//}
//return f, nil
//}

//// CreateFragments creates and stores all the fragments
//func (fb *FragmentBuilder) CreateFragments(p *elastic.BulkProcessor, nestFragments bool, compact bool) error {
//if (&r.Graph{}) == fb.Graph || fb.Graph.Len() == 0 {
//return fmt.Errorf("cannot store fragments from empty graph")
//}
//for t := range fb.Graph.IterTriples() {
//frag, err := fb.CreateFragment(t)
//if !compact {
//err := frag.AddHeader(fb)
//if err != nil {
//log.Printf("Unable to add header to fragment due to %v", err)
//return err
//}
//}
//if err != nil {
//log.Printf("Unable to create fragment due to %v.", err)
//return err
//}
//// nest fragments as opposed to using a parent child construction in ElasticSearch.
//// even though this would reduce the size of the index, it comes at the price of search performance.
//if nestFragments {
//fb.fg.Fragments = append(fb.fg.Fragments, frag)
//}
//}
//return nil
//}

// Doc returns the struct of the FragmentGraph object that is converted to a fragmentDoc record in ElasticSearch
func (fb *FragmentBuilder) Doc() *FragmentGraph {
	rm, err := fb.ResourceMap()
	if err != nil {
		log.Printf("Unable to create resources: %s", err)
		return fb.fg
	}
	err = rm.ResolveObjectIDs(fb.fg.Meta.HubID)
	if err != nil {
		log.Printf("Unable to resolve fragment resources: %s", err)
		return fb.fg
	}

	err = rm.SetContextLevels(fb.fg.GetAboutURI())
	if err != nil {
		log.Printf("Unable to set context: %s", err)
		return fb.fg
	}

	fb.fg.Resources = rm.ResourcesList()
	return fb.fg
}

// GetRDF returns a byte Array for the Flat JSON-LD serialized RDF
func (fb *FragmentBuilder) GetRDF() ([]byte, error) {
	var b bytes.Buffer
	err := fb.Graph.SerializeFlatJSONLD(&b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// IndexFragments updates the Fragments for standalone indexing and adds them to the Elasti BulkProcessorService
//func (fb *FragmentBuilder) IndexFragments(p *elastic.BulkProcessor) error {
//for _, frag := range fb.fg.Fragments {
//err := frag.AddHeader(fb)
//if err != nil {
//return err
//}
//frag.AddTo(p)
//}
//return nil
//}

// IsDomainExternal checks if the object link points to another domain
func (fb *FragmentBuilder) IsDomainExternal(obj string) (bool, error) {
	u, err := url.Parse(obj)
	if err != nil {
		return false, err
	}
	return !strings.Contains(c.Config.RDF.BaseURL, u.Host), nil
}

// IsGraphExternal checks if the object link points outside the current graph
func (fb *FragmentBuilder) IsGraphExternal(obj r.Term) bool {
	found := fb.Graph.One(obj, nil, nil)
	return found == nil
}

// ParseGraph creates a RDF2Go Graph
func (fb *FragmentBuilder) ParseGraph(rdf io.Reader, mimeType string) error {
	var err error
	switch mimeType {
	case "text/turtle":
		err = fb.Graph.Parse(rdf, mimeType)
	case "application/ld+json":
		err = fb.Graph.Parse(rdf, mimeType)
	default:
		return fmt.Errorf(
			"Unsupported RDF mimeType %s. Currently, only 'text/turtle' and 'application/ld+json' are supported",
			mimeType,
		)
	}
	if err != nil {
		log.Printf("Unable to parse RDF string into graph: %v\n%#v\n", err, rdf)
		return err
	}
	return nil
}