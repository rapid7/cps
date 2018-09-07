package s3

import (
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	log "github.com/sirupsen/logrus"

	kv "cps/pkg/kv"
)

var (
	Up     bool
	Health bool
	Config config
)

type config struct {
	bucket       string
	bucketRegion string
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.Print("starting v2 s3 watcher...")
}

func Poll(bucket, bucketRegion string) {
	Config = config{
		bucket:       bucket,
		bucketRegion: bucketRegion,
	}

	Sync()

	ticker := time.NewTicker(60 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				Sync()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func Sync() {
	log.Print("s3 sync begun")

	bucket := Config.bucket
	region := Config.bucketRegion

	svc := setUpAwsSession(region)
	resp, err := listBucket(bucket, svc)
	if err != nil {
		return
	}

	err = parseAllFiles(resp, bucket, svc)
	if err != nil {
		return
	}

	Up = true
	Health = true

	log.Print("S3 sync finished")
}

func setUpAwsSession(region string) s3iface.S3API {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	var svc s3iface.S3API = s3.New(sess)

	return svc
}

func listBucket(bucket string, svc s3iface.S3API) (*s3.ListObjectsOutput, error) {
	params := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	}

	resp, err := svc.ListObjects(params)
	if err != nil {
		log.Errorf("Error listing s3 objects %v:", err)
		Health = false
		return nil, err
	}

	return resp, nil
}

func parseAllFiles(resp *s3.ListObjectsOutput, bucket string, svc s3iface.S3API) error {
	var wg sync.WaitGroup
	wg.Add(len(resp.Contents))

	numCores := runtime.NumCPU()
	guard := make(chan struct{}, numCores*32)

	for _, key := range resp.Contents {
		guard <- struct{}{}
		go func(key *s3.Object) {
			defer wg.Done()
			parsePropertyFile(*key.Key, bucket, svc)
			<-guard
		}(key)
	}

	wg.Wait()

	return nil
}

func parsePropertyFile(k string, b string, svc s3iface.S3API) {
	isJSON, _ := regexp.Compile(".json$")

	if isJSON.MatchString(k) {
		result, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(b),
			Key:    aws.String(k),
		})

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
				log.Errorf("Download canceled due to timeout %v\n", err)
				Health = false
				return
			} else {
				log.Errorf("Failed to download object: %v", err)
				Health = false
				return
			}
		}

		body, err := ioutil.ReadAll(result.Body)
		if err != nil {
			log.Errorf("Failure to read body: %v\n", err)
			Health = false
			return
		}

		// Removes .json extension.
		path := k[0 : len(k)-5]

		kv.WriteProperty(path, body)

	} else {
		log.Printf("Skipping: %v.\n", k)
	}
}
