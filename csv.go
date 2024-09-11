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
	})
	for _, dashboard := range data.DashboardMap {
		rows = append(rows, []string{
			fmt.Sprintf("%d", dashboard.AccountId),
			dashboard.Guid,
			dashboard.Name,
			dashboard.Permalink,
			dashboard.CreatedBy,
		})
	}

	// Write CSV
	log.Printf("Writing csv %s", outputCSV)
	w := csv.NewWriter(f)
	err = w.WriteAll(rows)
	if err != nil {
		log.Printf("Error writing %s: %v", outputCSV, err)
	}
	w.Flush()
	f.Sync()
	f.Close()
}
