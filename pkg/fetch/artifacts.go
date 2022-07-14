package fetch

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
	OriginalArtifactsDir = "orig"
)

var (
	e2eFilesWhitelist = []string{"e2e-timelines_everything_"}
)

type JobRun struct {
	Job      string
	ID       string
	Path     string
	Started  time.Time
	Finished time.Time
}

// job "periodic-ci-openshift-release-master-ci-4.11-e2e-aws-upgrade-ovn-single-node", time.Date(2022, 7, 5, 0, 0, 0, 0, time.UTC)
func FetchJobArtifacts(job, workdir string) error {
	log.Printf("Fetching artifacts for job '%s'\n", job)

	r, err := getListOfJobRuns(context.Background(), job)
	if err != nil {
		return err
	}
	log.Printf("Obtained list of '%s''s runs (%d)\n", job, len(r))

	for _, run := range r {
		log.Printf("Fetching artifacts for %s: %s\n", job, run.ID)
		if err := fetchJobRunArtifact(run, workdir); err != nil {
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
	runs := []JobRun{}

	getTimestampFromFile := func(prefix, filename string) (time.Time, error) {
		rc, err := bucket.Object(prefix + filename).NewReader(ctx)
		if err != nil {
			return time.Time{}, err
		}
		defer rc.Close()

		buf := new(bytes.Buffer)
		buf.ReadFrom(rc)
		unixTimestamp := struct{ Timestamp int64 }{Timestamp: int64(0)}
		if err := json.Unmarshal(buf.Bytes(), &unixTimestamp); err != nil {
			return time.Time{}, err
		}
		return time.Unix(unixTimestamp.Timestamp, 0).UTC(), nil

	}

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

		var t time.Time
		_ = t

		started, err := getTimestampFromFile(attrs.Prefix, "started.json")
		if err != nil {
			return nil, err
		}
		finished, err := getTimestampFromFile(attrs.Prefix, "finished.json")
		if err != nil {
			if started.Unix() != 0 {
				// job started but didn't finished yet
				continue
			}
			return nil, err
		}

		prefixSplit := strings.Split(attrs.Prefix, "/")
		id := prefixSplit[len(prefixSplit)-1]
		if id == "" {
			id = prefixSplit[len(prefixSplit)-2]
		}
		runs = append(runs, JobRun{Job: jobName, ID: id, Path: attrs.Prefix, Started: started, Finished: finished})
	}

	return runs, nil
}

func fetchJobRunArtifact(run JobRun, workdir string) error {
	ctx := context.TODO()

	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return err
	}

	bucket := client.Bucket(bucketName)
	query := &storage.Query{Prefix: run.Path}

	saveTime := func(t time.Time, filepath string) error {
		ms, err := t.MarshalText()
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath, []byte(ms), 0644)
		if err != nil {
			return err
		}
		return nil
	}

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

		dir := path.Join(workdir, OriginalArtifactsDir, run.Job, run.ID)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}

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

		if err := saveTime(run.Started, path.Join(dir, "started")); err != nil {
			return err
		}
		if err := saveTime(run.Finished, path.Join(dir, "finished")); err != nil {
			return err
		}
	}

	return nil
}
