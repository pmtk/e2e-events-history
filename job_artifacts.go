package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/samber/lo"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"cloud.google.com/go/storage"
)

// gs://origin-ci-test/logs/periodic-ci-openshift-release-master-nightly-4.10-e2e-aws-single-node-serial/1503949008505147392/artifacts/e2e-aws-single-node-serial
// origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node/1545408471665479680
// origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node/1545408471665479680/started.json
// origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node/1545408471665479680/artifacts/e2e-aws-upgrade-ovn-single-node/single-node-e2e-test/artifacts/junit/

const (
	bucketName           = "origin-ci-test"
	bucketRootPath       = "logs"
	originalArtifactsDir = "orig"
)

var (
	e2eFilesWhitelist = []string{"e2e-timelines_everything_"}
)

type JobRun struct {
	Job     string
	ID      string
	Path    string
	Started time.Time
}

// job "periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node", time.Date(2022, 7, 5, 0, 0, 0, 0, time.UTC)
func fetchJobArtifacts(job string) error {
	log.Printf("Fetching artifacts for job '%s'\n", job)

	r, err := getListOfJobRuns(context.Background(), job)
	if err != nil {
		return err
	}
	log.Printf("Obtained list of '%s''s runs (%d)\n", job, len(r))

	for _, run := range r {
		log.Printf("Fetching artifacts for %s: %s\n", job, run.ID)
		if err := fetchJobRunArtifact(run); err != nil {
			log.Printf("Error when fetching artifacts for %s: %s: %+v\n", job, run.ID, err)
			return err
		}
	}
	return nil
}

func getListOfJobRuns(ctx context.Context, jobName string) ([]JobRun, error) {
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}

	bucket := client.Bucket(bucketName)
	query := &storage.Query{Delimiter: "/", Prefix: path.Join(bucketRootPath, jobName) + "/"}
	started := struct{ Timestamp int64 }{Timestamp: int64(0)}
	runs := []JobRun{}

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

		startedFile := bucket.Object(attrs.Prefix + "started.json")
		rc, err := startedFile.NewReader(ctx)
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(rc)
		if err := json.Unmarshal(buf.Bytes(), &started); err != nil {
			return nil, err
		}
		s := time.Unix(started.Timestamp, 0).UTC()

		prefixSplit := strings.Split(attrs.Prefix, "/")
		id := prefixSplit[len(prefixSplit)-1]
		if id == "" {
			id = prefixSplit[len(prefixSplit)-2]
		}
		runs = append(runs, JobRun{Job: jobName, ID: id, Path: attrs.Prefix, Started: s})
	}

	return runs, nil
}

func fetchJobRunArtifact(run JobRun) error {
	ctx := context.TODO()

	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return err
	}

	bucket := client.Bucket(bucketName)
	query := &storage.Query{Prefix: run.Path}

	it := bucket.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		if !strings.HasSuffix(attrs.Name, ".json") {
			continue
		}
		filename, err := lo.Last(strings.Split(attrs.Name, "/"))
		if err != nil {
			return err
		}
		if !lo.SomeBy(e2eFilesWhitelist, func(x string) bool { return strings.HasPrefix(filename, x) }) {
			continue
		}

		dir := path.Join(tmpWorkdir, originalArtifactsDir, run.Job, run.ID)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}

		// f := bucket.Object(attrs.Name)
		rc, err := bucket.Object(attrs.Name).NewReader(ctx)
		if err != nil {
			return err
		}
		defer rc.Close()

		file, _ := os.Create(path.Join(dir, filename))
		defer file.Close()

		writer := bufio.NewWriter(file)
		io.Copy(writer, rc)
		if err := writer.Flush(); err != nil {
			return err
		}
		ms, err := run.Started.MarshalText()
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(path.Join(dir, "started"), []byte(ms), 0644)
		if err != nil {
			return err
		}
	}

	return nil
}
