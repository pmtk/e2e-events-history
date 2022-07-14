package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
)

const (
	processedDir = "processed"
)

func processCachedData(job string) error {
	jobDir := path.Join(tmpWorkdir, originalArtifactsDir, job)
	if exists, err := fileExists(jobDir); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("couldn't find directory for job '%s', looked in '%s'", job, jobDir)
	}

	jobRunDirs, err := ioutil.ReadDir(jobDir)
	if err != nil {
		return err
	}

	for _, d := range jobRunDirs {
		if !d.IsDir() {
			continue
		}

		jobRunPath := path.Join(jobDir, d.Name())
		jobRunFiles, err := ioutil.ReadDir(jobRunPath)
		if err != nil {
			return err
		}

		eil := CIEventIntervalList{}
		var started time.Time

		for _, f := range jobRunFiles {
			path := path.Join(jobRunPath, f.Name())
			if f.Name() == "started" {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				if err := started.UnmarshalText(data); err != nil {
					return err
				}
				continue
			}

			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}

			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			intervals := &CIEventIntervalList{}
			if err := json.Unmarshal(data, intervals); err != nil {
				return err
			}
			eil.Merge(*intervals)
		}

		jes := eil.ToJobEvents(job, d.Name(), started)
		if err := jes.ToFile(path.Join(tmpWorkdir, processedDir, job, d.Name()+".json")); err != nil {
			return err
		}
	}

	return nil
}

type JobEvents struct {
	Job     string
	RunID   string
	Started time.Time
	Events  map[string]EventReduced
}

func (je *JobEvents) ToFile(filepath string) error {
	if err := os.MkdirAll(path.Dir(filepath), os.ModePerm); err != nil {
		return err
	}

	data, err := json.MarshalIndent(je, "", " ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (je *JobEvents) FromFile(p string) error {
	data, err := ioutil.ReadFile(p)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, je)
	if err != nil {
		return err
	}
	return nil
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

func (il *CIEventIntervalList) ToJobEvents(job, run string, started time.Time) *JobEvents {
	je := &JobEvents{Job: job, RunID: run, Started: started}

	filtered := lo.Filter(il.Items, func(ei CIEventInterval, _ int) bool {
		return strings.Contains(ei.Locator, "disruption") && strings.Contains(ei.Message, "stopped responding")
	})

	partitioned := PartitionBy(filtered, func(ei CIEventInterval) string {
		return ei.Locator
	})

	for _, v := range partitioned {
		sort.Slice(v, func(i, j int) bool {
			return v[i].From.Before(v[j].From)
		})
	}

	je.Events = lo.MapValues(partitioned,
		func(eis []CIEventInterval, _ string) EventReduced {
			return lo.Reduce(eis, func(er EventReduced, ei CIEventInterval, _ int) EventReduced {
				return er.AddInterval(Interval{From: ei.From, To: ei.To})
			}, EventReduced{Level: eis[0].Level, Locator: eis[0].Locator})
		})

	return je
}

type JobArtifacts struct {
	Started   time.Time
	ID        string
	Artifacts []string
}

type Interval struct {
	From     time.Time
	To       time.Time
	Duration float64
}

type EventReduced struct {
	Level         string
	Locator       string
	Intervals     []Interval
	TotalDuration float64
}

func (er EventReduced) AddInterval(i Interval) EventReduced {
	if len(er.Intervals) == 0 {
		er.Intervals = append(er.Intervals, Interval{From: i.From, To: i.To, Duration: i.To.Sub(i.From).Seconds()})
		return er
	}

	lastInterval := &er.Intervals[len(er.Intervals)-1]

	// https://en.wikipedia.org/wiki/Allen%27s_interval_algebra
	// Expecting only following for the disruptions:
	// - X precedes Y       == X.To  < Y.From  => 2 intervals
	// - X meets Y          == X.To == Y.From  => merge into one
	// - X overlaps with Y  == X.To  > Y.From  => merge into one (in case of merging multiple files)
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
