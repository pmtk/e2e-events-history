package process

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pmtk/e2e-events-history/pkg/fetch"
	"github.com/pmtk/e2e-events-history/pkg/helpers"
	"github.com/samber/lo"
)

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

		run, err := processOrigDir(path.Join(jobDir, d.Name()))
		if err != nil {
			return nil, err
		}
		run.ID = d.Name()
		j.Runs = append(j.Runs, *run)
	}

	return j, nil
}

func processOrigDir(runDirPath string) (*Run, error) {
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
