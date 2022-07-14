package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	jobsFile = "jobs.json"
)

type Jobs struct {
	Refresh time.Time
	Jobs    []string
}

func RefreshJobList(ctx context.Context, workdir string) error {
	jobs, err := getJobs(ctx)
	if err != nil {
		return err
	}
	if err := jobs.storeJobs(workdir); err != nil {
		return err
	}
	return nil
}

func StartPeriodicRefreshJobList(ctx context.Context, workdir string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := RefreshJobList(ctx, workdir); err != nil {
					fmt.Printf("Periodic job refresh failed: %+v\n", err)
				}
			}
		}
	}()

	return nil
}

func getJobs(ctx context.Context) (*Jobs, error) {
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}

	bucket := client.Bucket(bucketName)
	query := &storage.Query{Delimiter: "/", Prefix: bucketRootPath + "/periodic-ci-openshift-release-master"}
	query.SetAttrSelection([]string{"Prefix"})

	allJobs := Jobs{Refresh: time.Now().UTC()}

	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if attrs.Prefix == "" {
			continue
		}
		splPrefix := strings.Split(attrs.Prefix, "/")
		allJobs.Jobs = append(allJobs.Jobs, splPrefix[len(splPrefix)-2])
	}

	return &allJobs, nil
}

func (jobs *Jobs) storeJobs(workdir string) error {

	if err := os.MkdirAll(workdir, os.ModePerm); err != nil {
		return err
	}

	b, err := json.MarshalIndent(jobs, "", " ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(workdir, jobsFile), b, 0644)
	if err != nil {
		return err
	}
	return nil
}

func LoadJobsFromDisk(workdir string) (*Jobs, error) {
	b, err := ioutil.ReadFile(path.Join(workdir, jobsFile))
	if err != nil {
		return nil, err
	}

	jobs := &Jobs{}
	if err := json.Unmarshal(b, jobs); err != nil {
		return nil, err
	}

	return jobs, nil
}
