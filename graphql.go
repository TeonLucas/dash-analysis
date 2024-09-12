package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	GraphQlEndpoint = "https://api.newrelic.com/graphql"
	GrQl_Parallel   = 10
	DashboardQuery  = `query EntitySearchQuery($cursor: String) {actor {entitySearch(query: "domain = 'VIZ' AND type = 'DASHBOARD' AND accountId = %d", options: {tagFilter: ["createdBy","isDashboardPage"]}) {results(cursor: $cursor) {entities {guid accountId name permalink tags {key values}} nextCursor}}}}`
	DetailQuery     = `query getDashboard($guid: EntityGuid!) {actor {entity(guid: $guid) {... on DashboardEntity {name pages {name widgets {configuration {area {nrqlQueries {accountId query}} bar {nrqlQueries {accountId query}} billboard {nrqlQueries {accountId query}} line {nrqlQueries {accountId query}} markdown {text} pie {nrqlQueries {accountId query}} table {nrqlQueries {accountId query}}} id title} guid}}}}}`
)

// GraphQl request
type GraphQlPayload struct {
	Query     string `json:"query"`
	Variables struct {
		AccountId int    `json:"accountId,omitempty"`
		Guid      string `json:"guid,omitempty"`
		Cursor    string `json:"cursor,omitempty"`
	} `json:"variables"`
}

// GraphQl result
type GraphQlResult struct {
	Errors []Error `json:"errors"`
	Data   struct {
		Actor struct {
			Entity struct {
				Guid  string `json:"guid"`
				Name  string `json:"name"`
				Pages []struct {
					Guid       string      `json:"guid"`
					Name       string      `json:"name"`
					RawWidgets []RawWidget `json:"widgets"`
				}
			} `json:"entity"`
			EntitySearch struct {
				Results struct {
					Entities   []Entity    `json:"entities"`
					NextCursor interface{} `json:"nextCursor"`
				} `json:"results"`
			} `json:"entitySearch"`
		} `json:"actor"`
	} `json:"data"`
}
type RawWidget struct {
	Configuration struct {
		Area struct {
			NrqlQueries []NrqlQuery `json:"nrqlQueries"`
		} `json:"area"`
		Bar struct {
			NrqlQueries []NrqlQuery `json:"nrqlQueries"`
		} `json:"bar"`
		Billboard struct {
			NrqlQueries []NrqlQuery `json:"nrqlQueries"`
		} `json:"billboard"`
		Line struct {
			NrqlQueries []NrqlQuery `json:"nrqlQueries"`
		} `json:"line"`
		Markdown struct {
			NrqlQueries []NrqlQuery `json:"nrqlQueries"`
		} `json:"markdown"`
		Pie struct {
			NrqlQueries []NrqlQuery `json:"nrqlQueries"`
		} `json:"pie"`
		Table struct {
			NrqlQueries []NrqlQuery `json:"nrqlQueries"`
		} `json:"table"`
	} `json:"configuration"`
	Id    string `json:"id"`
	Title string `json:"title"`
}
type NrqlQuery struct {
	AccountId int    `json:"accountId"`
	Query     string `json:"query"`
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

// Cleaned up structures
type Dashboard struct {
	AccountId int    `json:"accountId"`
	Guid      string `json:"guid"`
	Name      string `json:"name"`
	Permalink string `json:"permalink"`
	CreatedBy string `json:"createdBy"`
	IsPage    bool   `json:"isPage"`
	WidgetMap map[int]Widget
	WidgetIds []int
}
type Widget struct {
	Id          int         `json:"id"`
	Title       string      `json:"title"`
	Guid        string      `json:"guid"`
	NrqlQueries []NrqlQuery `json:"nrqlQueries"`
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

	if len(entity.Tags) == 2 {
		for _, tag := range entity.Tags {
			if tag.Key == "createdBy" {
				if len(tag.Values) == 1 {
					dashboard.CreatedBy = tag.Values[0]
				}
			} else {
				if len(tag.Values) == 1 {
					dashboard.IsPage = tag.Values[0] == "true"
				}
			}
		}
	} else {
		log.Printf("Expected 2 tags, found %d on dashboard %s", len(entity.Tags), entity.Name)
	}
	return
}

func (data *LocalData) getDashboards() {
	var gQuery GraphQlPayload
	var j []byte
	var err error

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
			dashboard := parseEntity(entity)
			dashboard.WidgetMap = make(map[int]Widget)
			if dashboard.IsPage {
				// if it is a page, save it in the map
				data.DashboardMap[entity.Guid] = dashboard
			} else {
				// otherwise it is just a guid for the details query
				data.ParentGuids = append(data.ParentGuids, entity.Guid)
			}
		}
		if dashboardSearch.NextCursor == nil {
			break
		}
		// get next page of results
		gQuery.Variables.Cursor = fmt.Sprintf("%s", dashboardSearch.NextCursor)
	}
	log.Printf("Found %d dashboards, %d total pages", len(data.ParentGuids), len(data.DashboardMap))
}

func parseWidget(raw RawWidget) (widget Widget) {
	var err error
	widget.Id, err = strconv.Atoi(raw.Id)
	if err != nil {
		log.Printf("Error parsing widget id: %v", err)
	}
	widget.Title = raw.Title

	// Streamline structure
	if len(raw.Configuration.Area.NrqlQueries) > 0 {
		widget.NrqlQueries = raw.Configuration.Area.NrqlQueries
	} else if len(raw.Configuration.Bar.NrqlQueries) > 0 {
		widget.NrqlQueries = raw.Configuration.Bar.NrqlQueries
	} else if len(raw.Configuration.Billboard.NrqlQueries) > 0 {
		widget.NrqlQueries = raw.Configuration.Billboard.NrqlQueries
	} else if len(raw.Configuration.Line.NrqlQueries) > 0 {
		widget.NrqlQueries = raw.Configuration.Line.NrqlQueries
	} else if len(raw.Configuration.Pie.NrqlQueries) > 0 {
		widget.NrqlQueries = raw.Configuration.Pie.NrqlQueries
	} else if len(raw.Configuration.Table.NrqlQueries) > 0 {
		widget.NrqlQueries = raw.Configuration.Table.NrqlQueries
	}
	return
}

func (data *LocalData) getDashboardDetails() {
	inputChan := make(chan string, len(data.ParentGuids)+GrQl_Parallel)
	outputChan := make(chan Widget, len(data.DashboardMap)+GrQl_Parallel)

	// Load conditions into channel
	go func() {
		for _, guid := range data.ParentGuids {
			inputChan <- guid
		}
		for n := 0; n < GrQl_Parallel; n++ {
			inputChan <- ""
		}
	}()

	log.Printf("GraphQL - starting %d dashboard detail requestors", GrQl_Parallel)
	for i := 1; i <= GrQl_Parallel; i++ {
		go func() {
			var gQuery GraphQlPayload
			var j []byte
			var err error
			gQuery.Query = DetailQuery
			client := &http.Client{}
			for {
				guid := <-inputChan
				if len(guid) == 0 {
					outputChan <- Widget{}
					break
				}
				gQuery.Variables.Guid = guid
				// make query payload
				j, err = json.Marshal(gQuery)
				if err != nil {
					log.Printf("Error creating dashboard detail query: %v", err)
					continue
				}
				b := retryQuery(client, "POST", GraphQlEndpoint, string(j), data.GraphQlHeaders)
				// parse results
				var graphQlResult GraphQlResult
				err = json.Unmarshal(b, &graphQlResult)
				if err != nil {
					log.Printf("Error parsing dashboard detail result: %v", err)
					continue
				}
				if len(graphQlResult.Errors) > 0 {
					if graphQlResult.Errors[0].Message == "Not Found" {
						continue
					}
					log.Printf("Errors with dashboard detail query: %v", graphQlResult.Errors)
					continue
				}

				// Cleanup widgets and send to output channel
				for _, page := range graphQlResult.Data.Actor.Entity.Pages {
					log.Printf("Found dashboard page %s", page.Guid)
					for _, raw := range page.RawWidgets {
						widget := parseWidget(raw)
						if len(widget.NrqlQueries) == 0 {
							// Skip if markdown or no queries
							continue
						}
						widget.Guid = page.Guid
						outputChan <- widget
					}
				}
			}
		}()
	}

	widgets := 0
	for i := 0; i < GrQl_Parallel; i++ {
		for {
			output := <-outputChan
			if len(output.Guid) == 0 {
				break
			}
			dashboard, ok := data.DashboardMap[output.Guid]
			if !ok {
				log.Printf("Error with dashboard detail, no match for guid %d", output.Guid)
				continue
			}
			dashboard.WidgetMap[output.Id] = output
			dashboard.WidgetIds = append(dashboard.WidgetIds, output.Id)
			data.DashboardMap[output.Guid] = dashboard
			widgets++
		}
	}
	log.Printf("GraphQL - finished dashboard detail requesters, %d widgets found", widgets)
	for _, dashboard := range data.DashboardMap {
		sort.Ints(dashboard.WidgetIds)
		data.DashboardMap[dashboard.Guid] = dashboard
	}
}

func (data *LocalData) makeClient() {
	data.Client = &http.Client{}
	data.GraphQlHeaders = []string{"Content-Type:application/json", "API-Key:" + data.UserKey}
	data.DashboardMap = make(map[string]Dashboard)
}
