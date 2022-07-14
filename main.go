package main

import (
	"context"

	"github.com/pmtk/e2e-events-history/pkg/process"
)

// TODO: Refresh job artifacts on request (first check if new data)

const (
	tmpWorkdir = "workdir"
)

func main() {
	ctx := context.Background()
	_ = ctx

	// if err := fetch.RefreshJobList(ctx, tmpWorkdir); err != nil {
	// 	panic(err)
	// }
	// if err := fetch.StartPeriodicRefreshJobList(ctx, tmpWorkdir, time.Hour); err != nil {
	// 	panic(err)
	// }

	// go server.Start(ctx, tmpWorkdir)
	job := "periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node"
	// fetchJobArtifacts(job)

	if err := process.ProcessCachedJob(job, tmpWorkdir); err != nil {
		panic(err)
	}

	// <-ctx.Done()
}
