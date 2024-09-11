package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	GraphQlEndpoint = "https://api.newrelic.com/graphql"
	GrQl_Parallel   = 8
	DashboardQuery  = `query EntitySearchQuery($cursor: String) {actor {entitySearch(query: "domain = 'VIZ' AND type = 'DASHBOARD' AND accountId = %d", options: {tagFilter: ["createdBy"]}) {results(cursor: $cursor) {entities {guid accountId name permalink tags {key values}} nextCursor}}}}`
	DetailQuery     = `query getConditionDetail($accountId: Int!, $conditionId: ID!) {actor {account(id: $accountId) {alerts {nrqlCondition(id: $conditionId) {nrql {query} name id}}}}}`
)

type Dashboard struct {
	AccountId int    `json:"accountId"`
	Guid      string `json:"guid"`
	Name      string `json:"name"`
	Permalink string `json:"permalink"`
	CreatedBy string `json:"createdBy"`
	Query     string
}

// GraphQl request and result formats
type GraphQlPayload struct {
	Query     string `json:"query"`
	Variables struct {
		AccountId   int    `json:"accountId,omitempty"`
		ConditionId string `json:"conditionId,omitempty"`
		Cursor      string `json:"cursor,omitempty"`
	} `json:"variables"`
}
type GraphQlResult struct {
	Errors []Error `json:"errors"`
	Data   struct {
		Actor struct {
			EntitySearch struct {
				Results struct {
					Entities   []Entity    `json:"entities"`
					NextCursor interface{} `json:"nextCursor"`
				} `json:"results"`
			} `json:"entitySearch"`
		} `json:"actor"`
	} `json:"data"`
}
type Entity struct {
	AccountId int    `json:"accountId"`
	Guid      string `json:"guid"`
	Name      string `json:"name"`
	Permalink string `json:"permalink"`
	Tags      []struct {
		Key    string   `json:"key"`
		Values []string `json:"values"`
	} `json:"tags"`
}
type Error struct {
	Message string `json:"message"`
}

// Make API request with error retry
func retryQuery(client *http.Client, method, url, data string, headers []string) (b []byte) {
	var res *http.Response
	var err error
	var body io.Reader

	if len(data) > 0 {
		body = strings.NewReader(data)
	}

	// up to 3 retries on API error
	for j := 1; j <= 3; j++ {
		req, _ := http.NewRequest(method, url, body)
		for _, h := range headers {
			params := strings.Split(h, ":")
			req.Header.Set(params[0], params[1])
		}
		res, err = client.Do(req)
		if err != nil {
			log.Println(err)
		}
		if res != nil {
			if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusAccepted {
				break
			}
			log.Printf("Retry %d: http status %d", j, res.StatusCode)
		} else {
			log.Printf("Retry %d: no response", j)
		}
		time.Sleep(500 * time.Millisecond)
	}
	b, err = io.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
		return
	}
	res.Body.Close()
	return
}

func parseEntity(entity Entity) (dashboard Dashboard) {
	dashboard.AccountId = entity.AccountId
	dashboard.Guid = entity.Guid
	dashboard.Name = entity.Name
	dashboard.Permalink = entity.Permalink

	if len(entity.Tags) == 1 {
		if len(entity.Tags[0].Values) == 1 {
			dashboard.CreatedBy = entity.Tags[0].Values[0]
		}
	}
	return
}

func (data *LocalData) getDashboards() {
	var gQuery GraphQlPayload
	var j []byte
	var err error
	var conditionCount int

	// Get conditions, story in Policy map by guid
	gQuery.Query = fmt.Sprintf(DashboardQuery, data.AccountId)
	for {
		// make query payload
		j, err = json.Marshal(gQuery)
		if err != nil {
			log.Printf("Error creating GraphQl conditions query: %v", err)
		}
		b := retryQuery(data.Client, "POST", GraphQlEndpoint, string(j), data.GraphQlHeaders)

		// parse results
		var graphQlResult GraphQlResult
		log.Printf("Parsing GraphQl conditions response %d bytes", len(b))
		err = json.Unmarshal(b, &graphQlResult)
		if err != nil {
			log.Printf("Error parsing GraphQl conditions result: %v", err)
		}
		if len(graphQlResult.Errors) > 0 {
			log.Printf("Errors with GraphQl query: %v", graphQlResult.Errors)
		}
		dashboardSearch := graphQlResult.Data.Actor.EntitySearch.Results

		// store dashboards
		for _, entity := range dashboardSearch.Entities {
			data.DashboardMap[entity.Guid] = parseEntity(entity)
		}
		if dashboardSearch.NextCursor == nil {
			break
		}
		// get next page of results
		gQuery.Variables.Cursor = fmt.Sprintf("%s", dashboardSearch.NextCursor)
	}
	log.Printf("Found %d conditions", conditionCount)
}

func (data *LocalData) makeClient() {
	data.Client = &http.Client{}
	data.GraphQlHeaders = []string{"Content-Type:application/json", "API-Key:" + data.UserKey}
	data.DashboardMap = make(map[string]Dashboard)
}
