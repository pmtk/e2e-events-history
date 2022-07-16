package process

import (
	"sort"
	"time"

	"github.com/samber/lo"
)

func GetListOfMetricsForJobName(workdir, jobName string) ([]string, error) {
	job, err := LoadJob(workdir, jobName)
	if err != nil {
		return nil, err
	}

	return GetListOfMetricsForJob(job), nil
}

func GetListOfMetricsForJob(job *Job) []string {
	metrics := []string{}
	for _, run := range job.Runs {
		for k := range run.Events {
			metrics = append(metrics, k)
		}
	}
	sort.Strings(metrics)
	return lo.Uniq(metrics)
}

type IntervalASD struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Duration float64 `json:"duration"`
}

type RunMetricASD struct {
	ID            string        `json:"id"`
	Started       time.Time     `json:"started"`
	TotalDuration float64       `json:"totalDuration"`
	Intervals     []IntervalASD `json:"intervals"`
}

type JobASD struct {
	Name   string         `json:"name"`
	Metric string         `json:"metric"`
	Runs   []RunMetricASD `json:"runs"`
}

func LoadJobMetric(workdir, jobName, metric string) (*JobASD, error) {
	job, err := LoadJob(workdir, jobName)
	if err != nil {
		return nil, err
	}

	j := &JobASD{Name: job.Name, Metric: metric}

	for _, r := range job.Runs {
		rm := RunMetricASD{
			ID:            r.ID,
			Started:       r.Started,
			TotalDuration: r.Duration,
		}

		for key, event := range r.Events {
			if key == metric {
				rm.Intervals = lo.Map(event.Intervals, func(i Interval, _ int) IntervalASD {
					return IntervalASD{Start: i.Start, End: i.End, Duration: i.Duration}
				})
				break
			}
		}

		j.Runs = append(j.Runs, rm)
	}

	return j, nil
}
