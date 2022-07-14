package main

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

func initialAndPeriodicJobRefresh(ctx context.Context, interval time.Duration) error {
	refresh := func() error {
		jobs, err := getJobs(ctx)
		if err != nil {
			return err
		}
		if err := storeJobs(jobs); err != nil {
			return err
		}
		return nil
	}

	if err := refresh(); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := refresh(); err != nil {
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

func storeJobs(jobs *Jobs) error {

	if err := os.MkdirAll(tmpWorkdir, os.ModePerm); err != nil {
		return err
	}

	b, err := json.MarshalIndent(jobs, "", " ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(tmpWorkdir, jobsFile), b, 0644)
	if err != nil {
		return err
	}
	return nil
}

func loadJobsFromDisk() (*Jobs, error) {
	b, err := ioutil.ReadFile(path.Join(tmpWorkdir, jobsFile))
	if err != nil {
		return nil, err
	}

	jobs := &Jobs{}
	if err := json.Unmarshal(b, jobs); err != nil {
		return nil, err
	}

	return jobs, nil
}
