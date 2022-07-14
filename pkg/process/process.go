package process

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/samber/lo"
)

const (
	processedDir = "processed"
)

type Run struct {
	ID       string           `json:"id"`
	Started  time.Time        `json:"started"`
	Finished time.Time        `json:"finished"`
	Duration float64          `json:"duration"`
	Events   map[string]Event `json:"events"`
}

type Job struct {
	Name string `json:"name"`
	Runs []Run  `json:"runs"`
}

type Event struct {
	Level         string     `json:"level"`
	Locator       string     `json:"locator"`
	Intervals     []Interval `json:"intervals"`
	TotalDuration float64    `json:"totalDuration"`
}

type Interval struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Start    float64   `json:"start"`
	End      float64   `json:"end"`
	Duration float64   `json:"duration"`
}

func (er Event) AddInterval(i Interval) Event {
	if len(er.Intervals) == 0 {
		er.Intervals = append(er.Intervals, Interval{From: i.From, To: i.To, Duration: i.To.Sub(i.From).Seconds()})
		return er
	}

	lastInterval := &er.Intervals[len(er.Intervals)-1]

	// https://en.wikipedia.org/wiki/Allen%27s_interval_algebra
	// Expecting only following for the disruptions:
	// - X precedes Y       == X.To  < Y.From  => 2 intervals
	// - X meets Y          == X.To == Y.From  => merge into one
	// - X overlaps with Y  == X.To  > Y.From  => merge into one (shouldn't happen; just in case)
	switch {
	case lastInterval.To.Before(i.From):
		er.Intervals = append(er.Intervals, Interval{From: i.From, To: i.To, Duration: i.To.Sub(i.From).Seconds()})
	case lastInterval.To.Equal(i.From),
		lastInterval.To.After(i.From):

		lastInterval.To = i.To
		lastInterval.Duration = lastInterval.To.Sub(lastInterval.From).Seconds()
	}

	er.TotalDuration = lo.Reduce(lo.Map(er.Intervals, func(i Interval, _ int) float64 { return i.Duration }),
		func(total, dur float64, _ int) float64 {
			return total + dur
		}, 0.0)

	return er
}

func (er *Event) FillIntervalSecondsSinceJobStart(jobStart time.Time) {
	for i := range er.Intervals {
		er.Intervals[i].Start = er.Intervals[i].From.Sub(jobStart).Seconds()
		er.Intervals[i].End = er.Intervals[i].To.Sub(jobStart).Seconds()
	}
}

func LoadJob(workdir, jobName string) (*Job, error) {
	data, err := ioutil.ReadFile(path.Join(workdir, processedDir, jobName+".json"))
	if err != nil {
		return nil, err
	}
	j := &Job{}
	err = json.Unmarshal(data, j)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func ProcessCachedJob(jobName, workdir string) error {
	job, err := processCachedJob(jobName, workdir)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(job, "", " ")
	if err != nil {
		return err
	}
	filepath := path.Join(workdir, processedDir, jobName+".json")
	if err := os.MkdirAll(path.Dir(filepath), os.ModePerm); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath, data, 0644); err != nil {
		return err
	}

	return nil
}
