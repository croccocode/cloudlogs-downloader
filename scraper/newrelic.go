package scraper

import (
	"context"
	"fmt"
	"github.com/newrelic/newrelic-client-go/pkg/config"
	"github.com/newrelic/newrelic-client-go/pkg/nerdgraph"
	"github.com/newrelic/newrelic-client-go/pkg/region"
	"golang.org/x/time/rate"
	"strings"
	"time"
)

type NewRelicScraper struct {
	AccountId int
	Client    nerdgraph.NerdGraph

	// Nrql the query to run.  Omit SINCE, UNTILL and LIMIT MAX, as those keyword are appended automatically
	// during the query execution
	Nrql    string
	Limiter *rate.Limiter

	ParseLines ParseLineFunc
	MaxLines   int
}

type ParseLineFunc func(results []interface{}) ([]string, error)

func NewNerdGraphClient(apiKey string) nerdgraph.NerdGraph {
	r, _ := region.Get(region.EU)
	cfg := config.New()
	cfg.PersonalAPIKey = apiKey
	_ = cfg.SetRegion(r)
	client := nerdgraph.New(cfg)

	return client
}

func f(t time.Time) string {
	return t.Format("2006-01-02 15:04:05 MST")
}

func (r *NewRelicScraper) DoQuery(inst Inst) ([]string, error) {

	var lines []string

	// NewRelic does not paginate the responses
	// If a result set has more lines than the NRQL limit,
	// split the time interval and process it again
	intervals := []Inst{inst}
	for i := 0; i < len(intervals); i++ {
		currentInterval := intervals[i]

		linesIter, errQuery := r.doquery(currentInterval.From, currentInterval.To)
		if errQuery != nil {
			stderr.Printf("error evaluating interval, %s, %s, %v", f(currentInterval.From), f(currentInterval.To), errQuery)
			return nil, errQuery
		}

		// BUT do not split below 5seconds intervals,
		// to prevent infinite looping at 1second iintervals
		if len(linesIter) < r.MaxLines || inst.Duration() <= 5*time.Second {
			lines = append(lines, linesIter...)
			continue
		}

		stdout.Printf("partial results - interval will be splitted: %s, %s", f(currentInterval.From), f(currentInterval.To))
		newIntervals := SplitInterval(currentInterval)
		stdout.Printf("interval split in %s %s -> %s %s",
			newIntervals[0].From.Format("2006-01-02 15:04:05"),
			newIntervals[0].To.Format("2006-01-02 15:04:05"),
			newIntervals[1].From.Format("2006-01-02 15:04:05"),
			newIntervals[1].To.Format("2006-01-02 15:04:05"),
		)

		intervals = append(intervals, newIntervals...)
	}

	return lines, nil
}

func (r *NewRelicScraper) doquery(from, to time.Time) ([]string, error) {
	// self rate-limit the api before sending an api call
	ctx := context.Background()
	err := r.Limiter.WaitN(ctx, 1)
	if err != nil {
		stderr.Panic(err)
	}

	query := `
	query($accountId: Int!, $nrqlQuery: Nrql!) {
		actor {
			account(id: $accountId) {
				nrql(query: $nrqlQuery, timeout: 5) {
					results
				}
			}
		}
	}`

	// escapeNrql := strings.ReplaceAll(r.Nrql, "%", "%%")
	variables := map[string]interface{}{
		"accountId": r.AccountId,
		"nrqlQuery": fmt.Sprintf("%s SINCE '%s' UNTIL '%s' ORDER BY timestamp LIMIT MAX ", r.Nrql, from.Format("2006-01-02 15:04:05 MST"), to.Format("2006-01-02 15:04:05 MST")),
	}

	resp, err := r.Client.Query(query, variables)
	if err != nil {
		stderr.Printf("error query, %s, %s, %v", from.Format("2006-01-02 15:04:05 MST"), to.Format("2006-01-02 15:04:05 MST"), err)
		return nil, err
	}

	queryResp := resp.(nerdgraph.QueryResponse)
	actor := queryResp.Actor.(map[string]interface{})
	account := actor["account"].(map[string]interface{})
	nrql := account["nrql"].(map[string]interface{})
	results := nrql["results"].([]interface{})

	lines, err := r.ParseLines(results)

	return lines, nil
}

// ParseLogsUnique extract the log message from a NRQL query with UNIQUE()
func ParseLogsUnique(results []interface{}) ([]string, error) {
	var lines []string
	for _, r := range results {
		data := r.(map[string]interface{})

		messages := data["uniques.concat(timestamp, '|', message)"].([]interface{})
		for _, message := range messages {
			line := strings.TrimSpace(message.(string))
			line = strings.ReplaceAll(line, "\n", " ")
			lines = append(lines, line)
		}

	}

	return lines, nil
}

// ParsePodsLogs extract the log messages from an NRQL query
func ParsePodsLogs(results []interface{}) ([]string, error) {
	var lines []string
	for _, r := range results {
		data := r.(map[string]interface{})

		containerName := data["container_name"].(string)
		message := data["message"].(string)
		namespaceName := data["namespace_name"].(string)
		podName := data["pod_name"].(string)
		timestamp := data["timestamp"].(float64)

		message = strings.ReplaceAll(message, "\n", "")
		message = strings.ReplaceAll(message, "\"", "'")

		line := strings.Join([]string{
			fmt.Sprintf("%.0f", timestamp),
			namespaceName,
			containerName,
			podName,
			fmt.Sprintf("\"%s\"", message),
		}, ",")

		lines = append(lines, line)
	}

	return lines, nil
}
