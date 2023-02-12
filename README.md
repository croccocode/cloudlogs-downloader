# cloudlogs-downloader
![Build Status](https://github.com/croccocode/cloudlogs-downloader/actions/workflows/go.yml/badge.svg)
Bulk download logs from Newrelic (additional provider like AWS CloudWatch and Google Cloud will be added on request)

## Why do you need it
Many monitoring-as-a-service platform manage logs, like NewRelic, Grafana Cloud, AWS CloudWatch.
While it's easy to send logs to these platforms, getting them out is often not a given option. NewRelic does not support API request pagination, and each response is limited to 2000 rows. 
This measn that if you need to download 1 month of **your** logs for any reason, you can't. 

## How does it work
`cloudlogs-downloader` query NewRelic for logs, and automatically split the time range of your query in smaller interval of a given length. We call these intervals `steps`. Each step is downloaded in parallel. NewRelic does not paginate the responses, and returns up to 2000 rows.
If there are 2000 rows for a single step, the step is automatically cplit in half and the query executed again.

The inputs to `cloudlogs-downloader` are:
* The NewRelic `nrql` query to grap logs, eg `SELECT * FROM Logs WHERE clusterName = 'prod'`;
* The From and To interval you want to query;
* The size of the step, like 5 minutes;
* A destination folder;

Steps are saved in a local folder as `gzip` archive. If a step is already present in the destination folder, it is not downloaded again (so it is safe to stop this service in the middle of an execution)

## How to use it 
* Download the latest executable release;
* Copy and edit the `configuration.yaml` file
* Run `./cloudlogs-downloader` 