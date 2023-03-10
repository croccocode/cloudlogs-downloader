package scraper

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"testing"
	"time"
)

func TestCwQuery(t *testing.T) {
	sess, _ := session.NewSessionWithOptions(session.Options{
		Profile: "xxx",
		Config:  aws.Config{Region: aws.String("eu-west-1")},
	})

	runner := &CloudwatchLogsScraper{
		Cw:           cloudwatchlogs.New(sess),
		LogGroupName: "/aws/batch/job",
		Query: `fields @timestamp, @message, @logStream, @log
| parse @message /.+ (?<@time>\d+)/
| filter @time like /^164.+$/
| sort @timestamp desc
| limit 2000`,
	}

	start, _ := time.Parse("2006-01-02 15:04:05 MST", "2023-03-07 13:30:00 CET")
	end, _ := time.Parse("2006-01-02 15:04:05 MST", "2023-03-07 14:03:00 CET")

	lines, err := runner.DoQuery(Inst{
		From: start,
		To:   end,
	})

	if err != nil {
		t.Error(err)
	}
	println(len(lines))
}
