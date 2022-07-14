package main

import (
	"context"

	"github.com/pmtk/e2e-events-history/pkg/process"
	"github.com/pmtk/e2e-events-history/pkg/server"
)

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

	go server.Start(ctx, tmpWorkdir)
	job := "periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node"
	// fetchJobArtifacts(job)

	// _, err := getJobRunsToLoad(job, time.Date(2022, 07, 01, 0, 0, 0, 0, time.UTC), time.Date(2022, 7, 5, 0, 0, 0, 0, time.UTC))
	if err := process.ProcessCachedData(job, tmpWorkdir); err != nil {
		panic(err)
	}

	<-ctx.Done()
}
