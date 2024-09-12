package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
)

type LocalData struct {
	AccountId      int
	UserKey        string
	Client         *http.Client
	GraphQlHeaders []string
	CDPctx         context.Context
	CDPcancel      context.CancelFunc
	PolicyIds      []int
	DashboardMap   map[string]Dashboard
	ParentGuids    []string
	Dump           string
}

func main() {
	var err error

	// Get required settings
	data := LocalData{
		UserKey: os.Getenv("NEW_RELIC_USER_KEY"),
	}

	// Validate settings
	accountId := os.Getenv("NEW_RELIC_ACCOUNT")
	if len(accountId) == 0 {
		log.Printf("Please set env var NEW_RELIC_ACCOUNT")
		os.Exit(1)
	}
	data.AccountId, err = strconv.Atoi(accountId)
	if err != nil {
		log.Printf("Please set env var NEW_RELIC_ACCOUNT to an integer")
		os.Exit(1)
	}
	if len(data.UserKey) == 0 {
		log.Printf("Please set env var NEW_RELIC_USER_KEY")
		os.Exit(1)
	}
	data.makeClient()

	// Get list of dashboards
	data.getDashboards()

	// Get widgets and nrql
	data.getDashboardDetails()

	// Output CSV
	data.writeCSV()
	log.Println("Done")
}
