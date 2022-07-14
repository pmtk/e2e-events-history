package main

import (
	"context"

	"github.com/pmtk/e2e-events-history/pkg/process"
	"github.com/pmtk/e2e-events-history/pkg/server"
)

// TODO: Server expose a single page app using /jobs and /job/:name endpoints presenting a chart

// Later
// TODO: Refresh job artifacts on request (first check if new data)

const (
	tmpWorkdir = "workdir"
)

func main() {
	ctx := context.Background()

	// if err := fetch.RefreshJobList(ctx, tmpWorkdir); err != nil {
	// 	panic(err)
	// }
	// if err := fetch.StartPeriodicRefreshJobList(ctx, tmpWorkdir, time.Hour); err != nil {
	// 	panic(err)
	// }

	job := "periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node"
	if err := process.ProcessCachedJob(job, tmpWorkdir); err != nil {
		panic(err)
	}

	go server.Start(ctx, tmpWorkdir)
	<-ctx.Done()
}
