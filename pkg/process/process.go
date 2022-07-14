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
	From     time.Time
	To       time.Time
	Duration float64
}

type Event struct {
	Level         string
	Locator       string
	Intervals     []Interval
	TotalDuration float64
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
	ID      string
	Started time.Time
	Events  map[string]Event
}

type Job struct {
	Name string
	Runs []Run
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
	jobRunFiles, err := ioutil.ReadDir(runDirPath)
	if err != nil {
		return nil, err
	}

	eil := &CIEventIntervalList{}
	started := &time.Time{}

	for _, f := range jobRunFiles {
		path := path.Join(runDirPath, f.Name())
		if f.Name() == "started" {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if err := started.UnmarshalText(data); err != nil {
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

	r := &Run{Started: *started, Events: eil.ToMappedEvents()}

	return r, nil
}
