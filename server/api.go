package server

import (
	"fmt"
	"net/http"

	"bitbucket.org/delving/rapid/hub3"
	"bitbucket.org/delving/rapid/hub3/models"

	"github.com/asdine/storm"
	"github.com/kiivihal/goharvest/oai"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
)

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

// APIErrorMessage contains the default API error messages
type APIErrorMessage struct {
	HttpStatus int    `json:"code"`
	Message    string `json:"type"`
	Error      error  `json:error`
}

// bulkApi receives bulkActions in JSON form (1 per line) and processes them in
// ingestion pipeline.
func bulkAPI(c echo.Context) error {
	response, err := hub3.ReadActions(c.Request().Body)
	if err != nil {
		log.Info("Unable to read actions")
	}
	return c.JSON(http.StatusCreated, response)
}

// bindPMHRequest the query parameters to the OAI-Request
func bindPMHRequest(c echo.Context) oai.Request {
	r := c.Request()
	baseURL := fmt.Sprintf("http://%s%s", r.Host, r.URL.Path)
	req := oai.Request{
		Verb:            c.QueryParam("verb"),
		MetadataPrefix:  c.QueryParam("metadataPrefix"),
		Set:             c.QueryParam("set"),
		From:            c.QueryParam("from"),
		Until:           c.QueryParam("until"),
		Identifier:      c.QueryParam("identifier"),
		ResumptionToken: c.QueryParam("resumptionToken"),
		BaseURL:         baseURL,
	}
	return req
}

// oaiPmhEndpoint processed OAI-PMH request and returns the results
func oaiPmhEndpoint(c echo.Context) (err error) {
	req := bindPMHRequest(c)
	resp := hub3.ProcessVerb(&req)
	return c.XML(http.StatusOK, resp)
}

// listDataSets returns a list of all public datasets
func listDataSets(c echo.Context) (err error) {
	sets, err := models.ListDataSets()
	if err != nil {
		log.Printf("Unable to list datasets because of: %s", err)
		return err
	}
	return c.JSON(http.StatusOK, sets)
}

// getDataSet returns a dataset when found or a 404
func getDataSet(c echo.Context) error {
	ds, err := models.GetDataSet(c.Param("spec"))
	if err != nil {
		if err == storm.ErrNotFound {
			log.Printf("Unable to retrieve a dataset: %s", err)
			return c.JSON(http.StatusNotFound, APIErrorMessage{
				HttpStatus: http.StatusNotFound,
				Message:    fmt.Sprintf("%s was not found", c.Param("spec")),
				Error:      err,
			})
		}
		return err

	}
	return c.JSON(http.StatusOK, ds)
}

// createDataSet creates a new dataset.
func createDataSet(c echo.Context) error {
	spec := c.FormValue("spec")
	fmt.Printf("spec is %s", spec)
	ds, err := models.GetDataSet(spec)
	if err == storm.ErrNotFound {
		ds, err = models.CreateDataSet(spec)
		if err != nil {
			log.Printf("Unable to create dataset for %s", spec)
			return nil
		}
		return c.JSON(http.StatusCreated, ds)
	}
	return c.JSON(http.StatusNotModified, ds)
}
