package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pmtk/e2e-events-history/pkg/fetch"
	"github.com/pmtk/e2e-events-history/pkg/process"
)

const (
	historyUsage = "Example query: /history?job=NAME&events=image-registry[&days=60]"
)

func Start(ctx context.Context, workdir string) {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.GET("/jobs", func(c *gin.Context) {
		jobs, err := fetch.LoadJobsFromDisk(workdir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("Failed to get all jobs: %v", err)})
			return
		}
		c.JSON(http.StatusOK, jobs)
	})

	r.GET("/job/:name", func(c *gin.Context) {
		jobName := c.Param("name")
		log.Println(jobName)

		jobRunsData := &process.JobEvents{}
		jobRunsData.FromFile("")

		// jobs, err := loadJobsFromDisk()
		// if err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"reason": fmt.Sprintf("Failed to get all jobs: %v", err)})
		// 	return
		// }
		// c.JSON(http.StatusOK, jobs)
	})

	// r.GET("/history", func(c *gin.Context) {
	// 	jobName := c.Query("job")
	// 	events := c.Query("events")
	// 	days := c.DefaultQuery("days", "30")
	// 	fmt.Printf("jobName:%s | run:%s | days:%s\n", jobName, events, days)

	// 	if jobName == "" {
	// 		c.JSON(http.StatusBadRequest, gin.H{"reason": "Missing 'job' GET param. " + historyUsage})
	// 		return
	// 	}
	// 	if events == "" {
	// 		c.JSON(http.StatusBadRequest, gin.H{"reason": "Missing 'events' GET param. " + historyUsage})
	// 		return
	// 	}
	// 	c.JSON(http.StatusOK, gin.H{
	// 		"job":  jobName,
	// 		"run":  events,
	// 		"days": days,
	// 	})
	// })

	r.Run(":3000")
	<-ctx.Done()
}