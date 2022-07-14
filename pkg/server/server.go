package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pmtk/e2e-events-history/pkg/fetch"
	"github.com/pmtk/e2e-events-history/pkg/process"
	"github.com/samber/lo"
)

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
		log.Println(jobName)

		jobs, err := fetch.LoadJobsFromDisk(workdir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("failed to get all jobs: %v", err)})
			return
		}
		if !lo.Some(jobs.Jobs, []string{jobName}) {
			c.JSON(http.StatusBadRequest, gin.H{"reason": fmt.Sprintf("unknown job %s - get list of jobs at /jobs", jobName)})
			return
		}

		job, err := process.LoadJob(workdir, jobName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("failed to load job's data: %v", err)})
			return
		}
		c.JSON(http.StatusOK, job)
	})

	r.Run(":3000")
	<-ctx.Done()
}
