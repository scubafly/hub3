package models

import (
	"fmt"
	"time"

	. "bitbucket.org/delving/rapid/config"
)

// DataSet contains all the known informantion for a RAPID metadata dataset
type DataSet struct {
	//MapToPrefix string    `json:"mapToPrefix"`
	Spec     string    `json:"spec" storm:"id,index"`
	URI      string    `json:"uri" storm:"unique,index"`
	Revision int       `json:"revision"` // revision is used to mark the latest version of ingested RDFRecords
	Modified time.Time `json:"modified" storm:"index"`
	Created  time.Time `json:"created"`
	Deleted  bool      `json:"deleted"`
	Access   `storm:"inline"`
}

// Access determines the which types of access are enabled for this dataset
type Access struct {
	OAIPMH bool `json:"oaipmh"`
	Search bool `json:"search"`
	LOD    bool `json:"lod"`
}

// createDatasetURI creates a RDF uri for the dataset based Config RDF BaseUrl
func createDatasetURI(spec string) string {
	uri := fmt.Sprintf("%s/resource/dataset/%s", Config.RDF.BaseUrl, spec)
	return uri
}

// NewDataset creates a new instance of a DataSet
func NewDataset(spec string) DataSet {
	now := time.Now()
	access := Access{
		OAIPMH: true,
		Search: true,
		LOD:    true,
	}
	dataset := DataSet{
		Spec:     spec,
		URI:      createDatasetURI(spec),
		Created:  now,
		Modified: now,
		Access:   access,
	}
	return dataset
}

// Save saves the DataSet to BoltDB
func (ds DataSet) Save() error {
	ds.Modified = time.Now()
	return orm.Save(&ds)
}

// GetOrCreateDataSet returns a DataSet object from the Storm ORM.
// If none is present it will create one
func GetOrCreateDataSet(spec string) (DataSet, error) {
	var ds DataSet
	err := orm.One("Spec", spec, &ds)
	if err != nil {
		ds = NewDataset(spec)
		err = ds.Save()
	}
	return ds, nil
}

// IncrementRevision bumps the latest revision of the DataSet
func (ds *DataSet) IncrementRevision() error {
	ds.Revision++
	return ds.Save()
}

// ListDataSets returns an array of Datasets stored in Storm ORM
func ListDataSets() ([]DataSet, error) {
	var ds []DataSet
	err := orm.All(&ds)
	return ds, err
}