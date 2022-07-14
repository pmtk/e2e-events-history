package main

import (
	"context"
)

// TODO: Reply with events: { event, [{from, to, duration}], total-duration }
// TODO: Flatten processed files into one single JOB_NAME.json

const (
	tmpWorkdir = "workdir"
)

func main() {
	ctx := context.Background()
	_ = ctx

	// if err := initialAndPeriodicJobRefresh(ctx, time.Hour); err != nil {
	// 	panic(err)
	// }

	go server(ctx)
	job := "periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node"
	// fetchJobArtifacts(job)

	// _, err := getJobRunsToLoad(job, time.Date(2022, 07, 01, 0, 0, 0, 0, time.UTC), time.Date(2022, 7, 5, 0, 0, 0, 0, time.UTC))
	if err := processCachedData(job); err != nil {
		panic(err)
	}

	<-ctx.Done()
}
