package process

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pmtk/e2e-events-history/pkg/fetch"
	"github.com/pmtk/e2e-events-history/pkg/helpers"
	"github.com/samber/lo"
)

const (
	processedDir = "processed"
)

type Interval struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Start    float64   `json:"start"`
	End      float64   `json:"end"`
	Duration float64   `json:"duration"`
}

type Event struct {
	Level         string     `json:"level"`
	Locator       string     `json:"locator"`
	Intervals     []Interval `json:"intervals"`
	TotalDuration float64    `json:"totalDuration"`
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

type CIEventInterval struct {
	Level   string    `json:"level"`
	Locator string    `json:"locator"`
	Message string    `json:"message"`
	From    time.Time `json:"from"`
	To      time.Time `json:"to"`
}

type CIEventIntervalList struct {
	Items []CIEventInterval `json:"items"`
}

func (il *CIEventIntervalList) Merge(il2 CIEventIntervalList) {
	il.Items = append(il.Items, il2.Items...)
}

func (il *CIEventIntervalList) ToMappedEvents() map[string]Event {
	filtered := lo.Filter(il.Items, func(ei CIEventInterval, _ int) bool {
		return strings.Contains(ei.Locator, "disruption") && strings.Contains(ei.Message, "stopped responding")
	})

	partitioned := helpers.PartitionBy(filtered, func(ei CIEventInterval) string {
		return ei.Locator
	})

	for _, v := range partitioned {
		sort.Slice(v, func(i, j int) bool {
			return v[i].From.Before(v[j].From)
		})
	}

	return lo.MapValues(partitioned,
		func(eis []CIEventInterval, _ string) Event {
			return lo.Reduce(eis, func(er Event, ei CIEventInterval, _ int) Event {
				return er.AddInterval(Interval{From: ei.From, To: ei.To})
			}, Event{Level: eis[0].Level, Locator: eis[0].Locator})
		})
}

type Run struct {
	ID       string           `json:"id"`
	Started  time.Time        `json:"started"`
	Finished time.Time        `json:"finished"`
	Duration float64          `json:"duration"`
	Events   map[string]Event `json:"events"`
}

type Job struct {
	Name string `json:""`
	Runs []Run  `json:""`
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

func processCachedJob(jobName, workdir string) (*Job, error) {
	jobDir := path.Join(workdir, fetch.OriginalArtifactsDir, jobName)
	if _, err := helpers.FileExists(jobDir); err != nil {
		return nil, err
	}

	runDirs, err := ioutil.ReadDir(jobDir)
	if err != nil {
		return nil, err
	}

	j := &Job{Name: jobName}

	for _, d := range runDirs {
		if !d.IsDir() {
			continue
		}

		run, err := loadOrigRunFiles(path.Join(jobDir, d.Name()))
		if err != nil {
			return nil, err
		}
		run.ID = d.Name()
		j.Runs = append(j.Runs, *run)
	}

	return j, nil
}

func loadOrigRunFiles(runDirPath string) (*Run, error) {
	readTimestampFile := func(filepath string, t *time.Time) error {
		data, err := ioutil.ReadFile(filepath)
		if err != nil {
			return err
		}
		if err := t.UnmarshalText(data); err != nil {
			return err
		}
		return nil
	}

	jobRunFiles, err := ioutil.ReadDir(runDirPath)
	if err != nil {
		return nil, err
	}

	eil := &CIEventIntervalList{}

	r := &Run{}
	for _, f := range jobRunFiles {
		path := path.Join(runDirPath, f.Name())

		if f.Name() == "started" {
			if err := readTimestampFile(path, &r.Started); err != nil {
				return nil, err
			}
			continue
		}
		if f.Name() == "finished" {
			if err := readTimestampFile(path, &r.Finished); err != nil {
				return nil, err
			}
			continue
		}

		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		intervals := &CIEventIntervalList{}
		if err := json.Unmarshal(data, intervals); err != nil {
			return nil, err
		}
		eil.Merge(*intervals)
	}

	r.Duration = r.Finished.Sub(r.Started).Seconds()
	r.Events = eil.ToMappedEvents()
	for _, v := range r.Events {
		v.FillIntervalSecondsSinceJobStart(r.Started)
	}
	return r, nil
}
