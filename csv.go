package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
)

func (data *LocalData) writeCSV() {
	var rows [][]string

	outputCSV := fmt.Sprintf("dashboards_%d.csv", data.AccountId)
	f, err := os.Create(outputCSV)
	if err != nil {
		log.Printf("Error opening csv: %v", err)
	}

	// Make rows
	rows = append(rows, []string{
		"accountId",
		"guid",
		"name",
		"permalink",
		"createdBy",
		"chartId",
		"chartName",
		"nrqlAccountId",
		"nrqlQuery",
	})
	for _, dashboard := range data.DashboardMap {
		//log.Printf("Found Dashboard %s, page=%v", dashboard.Guid, dashboard.IsPage)
		for _, widgetId := range dashboard.WidgetIds {
			widget := dashboard.WidgetMap[widgetId]
			for _, query := range widget.NrqlQueries {
				rows = append(rows, []string{
					fmt.Sprintf("%d", dashboard.AccountId),
					dashboard.Guid,
					dashboard.Name,
					dashboard.Permalink,
					dashboard.CreatedBy,
					fmt.Sprintf("%d", widgetId),
					widget.Title,
					fmt.Sprintf("%d", query.AccountId),
					query.Query,
				})
			}
		}
	}

	// Write CSV
	log.Printf("Writing csv %s with %d entries", outputCSV, len(rows)-1)
	w := csv.NewWriter(f)
	err = w.WriteAll(rows)
	if err != nil {
		log.Printf("Error writing %s: %v", outputCSV, err)
	}
	w.Flush()
	f.Sync()
	f.Close()
}
