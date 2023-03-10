package scraper

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
	"log"
	"os"
	"sync"
	"time"
)

var (
	stdout = log.New(os.Stdout, "", log.Lshortfile|log.LstdFlags)
	stderr = log.New(os.Stderr, "[ERROR] ", log.Lmsgprefix|log.Lshortfile|log.LstdFlags)

	errTooManyRows = errors.New("too many rows")
)

type Inst struct {
	From time.Time
	To   time.Time
}

func (i Inst) Duration() time.Duration {
	return i.To.Sub(i.From)
}

type LogScraper interface {
	DoQuery(inst Inst) ([]string, error)
}

func SetStdout(logger *log.Logger) {
	stdout = logger
}

func SetStderr(logger *log.Logger) {
	stderr = logger
}

type StdoutTest struct {
}

func (r *StdoutTest) DoQuery(from, to time.Time) []string {
	stdout.Printf("starting test")
	println(fmt.Sprintf("from %s to %s", from.String(), to.String()))
	return nil
}

func WriteFileGzip(fileName string, lines []string) {
	// Open file
	f, err := os.Create(fileName)
	if err != nil {
		panic(err)
	}
	gzWriter := gzip.NewWriter(f)

	defer func() {
		_ = gzWriter.Close()
		_ = f.Close()
	}()

	for _, line := range lines {
		_, err = gzWriter.Write([]byte(line))
		if err != nil {
			panic(err)
		}

		_, err = gzWriter.Write([]byte("\n"))
		if err != nil {
			panic(err)
		}
	}

}

func SplitInterval(inst Inst) []Inst {
	interval := inst.To.Sub(inst.From).Seconds()
	halfInterval := interval / 2
	period := time.Second

	return []Inst{
		{
			From: inst.From,
			To:   inst.From.Add(time.Duration(halfInterval) * period),
		},
		{
			From: inst.From.Add(time.Duration(halfInterval) * period),
			To:   inst.To,
		},
	}
}

// ScrapeLogs starts querying the log provider using the
func ScrapeLogs(start, end time.Time, step time.Duration, runner LogScraper, destinationPath string, limiter *rate.Limiter) {

	inst := start.Add(step)
	instChannel := make(chan Inst, 500)
	wg := sync.WaitGroup{}

	d := end.Sub(start).Minutes()
	steps := d / step.Minutes()
	bar := progressbar.Default(int64(steps))

	// start the worker
	nThread := 3
	for i := 0; i < nThread; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			processor(instChannel, destinationPath, bar, limiter, runner)
		}()
	}

	// generate time slices
	iter := 0.0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for inst.Before(end) || inst.Equal(end) {
			// log.Printf("queueing from %s to %s", start.String(), end.String())
			instChannel <- Inst{From: start, To: inst}

			// update the progress bar - how to waste my time. Is it  a waste of time?
			iter += 1
			if iter > steps {
				bar.ChangeMax(int(iter))
			}

			start = start.Add(step)
			inst = start.Add(step)
		}
		close(instChannel)

	}()

	wg.Wait()

}

func processor(instChannel chan Inst, destinationPath string, bar *progressbar.ProgressBar, limiter *rate.Limiter, runner LogScraper) {
	for inst := range instChannel {
		inst := inst
		// skip if the file exists (without creating a routine)
		fileName := fmt.Sprintf("%s/%s-%s.csv.gz", destinationPath, inst.From.Format("20060102_150405"), inst.To.Format("20060102_150405"))
		_, err := os.Stat(fileName)
		if err == nil {
			// stdout.Printf("%s exists, skipping", fileName)
			_ = bar.Add(1)
			continue
		}

		// If a result set has more lines than the max number of rows limit,
		// split the time interval and process it again
		intervals := []Inst{inst}
		for i := 0; i < len(intervals); i++ {
			err = limiter.WaitN(context.Background(), 1)
			if err != nil {
				panic(err)
			}

			currentInterval := intervals[i]
			lines, err := runner.DoQuery(currentInterval)

			if err != nil {
				if errors.Is(err, errTooManyRows) {
					newIntervals := SplitInterval(currentInterval)
					intervals = append(intervals, newIntervals...)
					continue
				}

				stderr.Printf("error downoading logs: %v", err)
				continue
			}

			WriteFileGzip(fileName, lines)
			_ = bar.Add(1)

		}

	}
}
