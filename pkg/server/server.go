package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pmtk/e2e-events-history/pkg/fetch"
	"github.com/pmtk/e2e-events-history/pkg/process"
	"github.com/samber/lo"
)

func getJobIfNameValid(workdir, jobName string, c *gin.Context) *process.Job {
	jobs, err := fetch.LoadJobsFromDisk(workdir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("failed to get all jobs: %v", err)})
		return nil
	}
	if !lo.Some(jobs.Jobs, []string{jobName}) {
		c.JSON(http.StatusBadRequest, gin.H{"reason": fmt.Sprintf("unknown job %s - get list of jobs at /jobs", jobName)})
		return nil
	}
	job, err := process.LoadJob(workdir, jobName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("failed to load job's data: %v", err)})
		return nil
	}
	return job
}

func Start(ctx context.Context, workdir string) {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.GET("/jobs", func(c *gin.Context) {
		jobs, err := fetch.LoadJobsFromDisk(workdir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("failed to get all jobs: %v", err)})
			return
		}
		c.JSON(http.StatusOK, jobs)
	})

	r.GET("/job/:name", func(c *gin.Context) {
		jobName := c.Param("name")

		job := getJobIfNameValid(workdir, jobName, c)
		if job == nil {
			return
		}

		d := struct {
			Metrics []string `json:"metrics"`
		}{
			Metrics: process.GetListOfMetricsForJob(job),
		}
		c.JSON(http.StatusOK, d)
	})

	r.GET("/job/:name/*metric", func(c *gin.Context) {
		jobName := c.Param("name")
		metricName := strings.TrimPrefix(c.Param("metric"), "/") // aka disruption for now

		j, err := process.LoadJobMetric(workdir, jobName, metricName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("failed to load metric: %v", err)})
			return
		}

		c.JSON(http.StatusOK, j.Runs)
	})

	r.GET("/", func(c *gin.Context) {
		c.File("html/index.html")
	})

	r.Run(":3000")
	<-ctx.Done()
}
