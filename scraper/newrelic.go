package scraper

import (
	"fmt"
	"github.com/newrelic/newrelic-client-go/pkg/config"
	"github.com/newrelic/newrelic-client-go/pkg/nerdgraph"
	"github.com/newrelic/newrelic-client-go/pkg/region"
	"strings"
	"time"
)

type NewRelicScraper struct {
	AccountId int
	Client    nerdgraph.NerdGraph

	// Nrql the query to run.  Omit SINCE, UNTILL and LIMIT MAX, as those keyword are appended automatically
	// during the query execution
	Nrql     string
	Fields   []string
	MaxLines int
}

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

	lines, errQuery := r.doquery(inst.From, inst.To)
	if errQuery != nil {
		stderr.Printf("error evaluating interval, %s, %s, %v", f(inst.From), f(inst.To), errQuery)
		return nil, errQuery
	}

	// NewRelic does not paginate the responses
	// If a result set has more lines than the NRQL limit,
	// split the time interval and process it again
	// BUT do not split below 5seconds intervals,
	// to prevent infinite looping at 1second iintervals
	if len(lines) >= r.MaxLines && inst.Duration() > 5*time.Second {
		return lines, errTooManyRows
	}

	return lines, nil
}

func (r *NewRelicScraper) doquery(from, to time.Time) ([]string, error) {
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
	var lines []string
	// len(data)
	for _, row := range results {
		data := row.(map[string]interface{})
		items := make([]string, len(r.Fields))
		for i, key := range r.Fields {
			_, gotKey := data[key]
			if !gotKey {
				// The key is missing in the resultset.
				// Use an empty string to keep the csv formatted
				items[i] = ""
				continue
			}
			l := ""
			stringVal, isString := data[key].(string)
			floatVal, isFloat := data[key].(float64)
			if isFloat {
				l = fmt.Sprintf("%f", floatVal)
			} else if isString {
				l = stringVal
			} else {
				// panic and see what type is
				stderr.Printf("problem with key: %s", key)
				_ = data[key].(string)
			}

			l = cleanString(l)
			items[i] = l
		}

		lines = append(lines, strings.Join(items, ","))
	}

	return lines, nil
}

func cleanString(input string) string {
	s := strings.ReplaceAll(input, "\n", "")
	s = strings.ReplaceAll(s, "\"", "'")
	return s
}
