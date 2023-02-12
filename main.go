package main

import (
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
		stderr.Panicf("fatal error config file: %w", err)
	}

	start, _ := time.Parse("2006-01-02 15:04:05 MST", viper.GetString("from"))
	end, _ := time.Parse("2006-01-02 15:04:05 MST", viper.GetString("to"))
	step := time.Duration(viper.GetInt("step")) * time.Second
	destinationPath := viper.GetString("destinationPath")

	nrAccountNum := viper.GetInt("newrelic.queryAccountNumber")
	nrApiKey := viper.GetString("newrelic.apiKey")
	limitBuckSize := viper.GetInt("newrelic.maxCallPerSec")
	runner := &scraper.NewRelicScraper{
		AccountId:  nrAccountNum, // pr-factory
		Nrql:       viper.GetString("newrelic.nrql"),
		Limiter:    rate.NewLimiter(rate.Limit(limitBuckSize), 30),
		Client:     scraper.NewNerdGraphClient(nrApiKey),
		ParseLines: scraper.ParsePodsLogs,
		MaxLines:   2000,
	}

	scraper.ScrapeLogs(start, end, step, runner, destinationPath)

}
