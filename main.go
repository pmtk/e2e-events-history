package main

import (
	"context"
	"fmt"
	"log"

	"github.com/pmtk/e2e-events-history/pkg/fetch"
	"github.com/pmtk/e2e-events-history/pkg/process"
	"github.com/pmtk/e2e-events-history/pkg/server"
)

// TODO: cleanup structs in process pkg
// TODO: provide info in metrics.json if job made all steps (didn't end prematurely)
// TODO: alerts, operator-{unavailable,degraded,progressing}, endpoint-availability
// TODO: ha: think about node's IPs - how to transform them into {master,worker}-{0,1,2}: node-state, reboots
// TODO: sno: reboot as a metric, some of disruption will happen in the same time
// TODO: don't re-fetch and re-process the data if exists
// TODO: refresh job artifacts on trigger (cron, form-request)
// TODO: database: nosql?

const (
	tmpWorkdir = "workdir"
)

func main() {
	fmt.Printf("Starting\n")
	ctx := context.Background()

	// fmt.Printf("Fetching list of jobs...")
	// if err := fetch.RefreshJobList(ctx, tmpWorkdir); err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("\tdone\n")
	// if err := fetch.StartPeriodicRefreshJobList(ctx, tmpWorkdir, time.Hour); err != nil { panic(err) }

	jobs := []string{
		"periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node",
		"periodic-ci-openshift-release-master-nightly-4.11-e2e-aws-single-node",
	}
	for _, job := range jobs {
		fmt.Printf("Fetching artifacts for %s...\n", job)
		if err := fetch.FetchJobArtifacts(job, tmpWorkdir); err != nil {
			log.Panicf("%+v\n", err)
		}

		fmt.Printf("Processing artifacts for %s...", job)
		if err := process.ProcessCachedJob(job, tmpWorkdir); err != nil {
			log.Panicf("%+v\n", err)
		}
		fmt.Printf("\tdone\n")
	}

	go server.Start(ctx, tmpWorkdir)
	<-ctx.Done()
}
