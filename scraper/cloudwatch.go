package scraper

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"strings"
)

type CloudwatchLogsScraper struct {
	AwsProfile string
	AwsRegion  string

	Query        string
	Cw           *cloudwatchlogs.CloudWatchLogs
	LogGroupName string
}

func (r *CloudwatchLogsScraper) DoQuery(inst Inst) ([]string, error) {
	startTimeEpoch := inst.From.Unix()
	endTimeEpoch := inst.To.Unix()
	nMaxLines := 10000

	startQuery, err := r.Cw.StartQuery(&cloudwatchlogs.StartQueryInput{
		StartTime:     aws.Int64(startTimeEpoch),
		EndTime:       aws.Int64(endTimeEpoch),
		Limit:         aws.Int64(int64(nMaxLines)),
		LogGroupNames: []*string{aws.String(r.LogGroupName)},
		QueryString:   aws.String(r.Query),
	})

	if err != nil {
		return nil, err
	}

	var q *cloudwatchlogs.GetQueryResultsOutput
	for {
		q, err = r.Cw.GetQueryResults(&cloudwatchlogs.GetQueryResultsInput{QueryId: startQuery.QueryId})
		if err != nil {
			return nil, err
		}

		status := *q.Status
		if status == "Running" || status == "Scheduled" {
			stdout.Printf("query is %s, continue", status)
			continue
		}

		if status == "Cancelled" || status == "Failed" || status == "Timeout" || status == "Unknown" {
			stderr.Printf("query %s failed with status: %s - skip", *startQuery.QueryId, status)
			return nil, errors.New("query failed or incomplete")
		}

		if status != "Complete" {
			stderr.Panicf("unknows status for query %s : %s", *startQuery.QueryId, status)
		}
		break

	}

	var lines []string
	for _, result := range q.Results {
		timestamp := ""
		message := ""
		for _, field := range result {
			if *field.Field == "@timestamp" {
				timestamp = *field.Value
			} else if *field.Field == "@message" {
				message = *field.Value
			}
		}

		lines = append(lines, strings.Join([]string{timestamp, message}, ","))
	}

	if len(lines) >= nMaxLines {
		return lines, errTooManyRows
	}
	return lines, nil
}
