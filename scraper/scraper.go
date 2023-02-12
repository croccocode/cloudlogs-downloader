package scraper

import (
	"compress/gzip"
	"fmt"
	"github.com/schollz/progressbar/v3"
	"log"
	"os"
	"sync"
	"time"
)

var (
	stdout *log.Logger
	stderr *log.Logger
)

type Inst struct {
	From time.Time
	To   time.Time
}

func (i Inst) Duration() time.Duration {
	return i.To.Sub(i.From)
}

type LogScraper interface {
	DoQuery(from, to time.Time) ([]string, error)
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
func ScrapeLogs(start, end time.Time, step time.Duration, runner *NewRelicScraper, destinationPath string) {

	inst := start.Add(step)
	instChannel := make(chan Inst, 500)
	wg := sync.WaitGroup{}

	d := end.Sub(start).Minutes()
	steps := d / step.Minutes()
	bar := progressbar.Default(int64(steps))

	// start the worker
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := range instChannel {
			i := i
			// skip if the file exists (without creating a routine)
			fileName := fmt.Sprintf("%s/%s-%s.csv.gz", destinationPath, i.From.Format("20060102_150405"), i.To.Format("20060102_150405"))
			_, err := os.Stat(fileName)
			if err == nil {
				// stdout.Printf("%s exists, skipping", fileName)
				_ = bar.Add(1)
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()

				lines, err := runner.DoQuery(i)
				if err == nil {
					WriteFileGzip(fileName, lines)
				}
				_ = bar.Add(1)

			}()
		}
	}()

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
