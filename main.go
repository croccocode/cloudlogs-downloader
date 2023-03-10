package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/spf13/viper"
	"gitlab.mgmt.infocert.it/infra/nrlog-exporter/scraper"
	"golang.org/x/time/rate"
	"io"
	"log"
	"os"
	"time"
)

var (
	stdout *log.Logger
	stderr *log.Logger
)

func init() {

	stdoutFile, _ := os.OpenFile("info.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	stderrFile, _ := os.OpenFile("errors.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)

	stdout = log.New(io.MultiWriter(stdoutFile), "", log.Lshortfile|log.LstdFlags)
	stderr = log.New(io.MultiWriter(stderrFile, os.Stderr), "[ERROR] ", log.Lmsgprefix|log.Lshortfile|log.LstdFlags)

	scraper.SetStdout(stdout)
	scraper.SetStderr(stderr)

}

func main() {

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		stderr.Panicf("fatal error config file: %v", err)
	}

	start, _ := time.Parse("2006-01-02 15:04:05 MST", viper.GetString("from"))
	end, _ := time.Parse("2006-01-02 15:04:05 MST", viper.GetString("to"))
	step := time.Duration(viper.GetInt("step")) * time.Second
	destinationPath := viper.GetString("destinationPath")

	var logScraper scraper.LogScraper
	limiter := rate.NewLimiter(rate.Limit(viper.GetInt("maxCallPerSec")), 30)
	nrAccountNum := viper.GetInt("newrelic.queryAccountNumber")
	awsProfile := viper.GetString("cloudwatch.awsProfile")

	if nrAccountNum != 0 && awsProfile != "" {
		stderr.Panicf("please specify only one between NewRelic and Cloudwatch ")
	}

	if nrAccountNum != 0 {
		nrApiKey := viper.GetString("newrelic.apiKey")

		runner := &scraper.NewRelicScraper{
			AccountId:  nrAccountNum, // pr-factory
			Nrql:       viper.GetString("newrelic.nrql"),
			Client:     scraper.NewNerdGraphClient(nrApiKey),
			ParseLines: scraper.ParsePodsLogs,
			MaxLines:   2000,
		}
		logScraper = runner
	}

	if awsProfile != "" {

		awsRegion := viper.GetString("cloudwatch.awsRegion")
		sess, err := session.NewSessionWithOptions(session.Options{
			Profile: awsProfile,
			Config:  aws.Config{Region: aws.String(awsRegion)},
		})
		if err != nil {
			stderr.Panic(err)
		}

		runner := &scraper.CloudwatchLogsScraper{
			Cw:           cloudwatchlogs.New(sess),
			LogGroupName: viper.GetString("cloudwatch.logGroup"),
			Query:        viper.GetString("cloudwatch.query"),
		}
		logScraper = runner
	}

	if logScraper == nil {
		stderr.Panicf("no runner configured")
	}
	scraper.ScrapeLogs(start, end, step, logScraper, destinationPath, limiter)

}
