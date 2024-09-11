# dash-analysis
Generates a CSV to analyze dashboards in a given account


## To build
Compile with
```
go build
```

## To configure
First you need to set two environment variables:
```
export NEW_RELIC_ACCOUNT=YOUR_TARGET_ACCOUNT_ID
export NEW_RELIC_USER_KEY=YOUR_USER_API_KEY
```

## To run
Then run as follows
```
./dash-analysis
```

