package s3

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/peterbourgon/mergemap"

	log "github.com/sirupsen/logrus"

	index "cps/pkg/index"
	kv "cps/pkg/kv"
)

var (
	Up     bool
	Health bool
	Config config
	isJSON = regexp.MustCompile(".json$")
	mu     = sync.Mutex{}
)

type config struct {
	bucket       string
	bucketRegion string
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
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
	resp, err := listBucket(bucket, region, svc)
	if err != nil {
		log.Error(err)
		return
	}

	if err := parseAllFiles(resp, bucket, svc); err != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()
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

func listBucket(bucket, region string, svc s3iface.S3API) ([]*s3.ListObjectsOutput, error) {

	i, err := index.ParseIndex(bucket, region)
	if err != nil {
		return nil, err
	}

	var responses []*s3.ListObjectsOutput

	for _, prefix := range i {
		params := &s3.ListObjectsInput{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		}

		resp, err := svc.ListObjects(params)
		if err != nil {
			log.Errorf("Error listing s3 objects %v:", err)
			Health = false
			return nil, err
		}

		responses = append(responses, resp)
	}

	return responses, nil
}

func parseAllFiles(resp []*s3.ListObjectsOutput, bucket string, svc s3iface.S3API) error {

	var files []string

	for _, object := range resp {
		for _, key := range object.Contents {
			files = append(files, *key.Key)
		}
	}

	if err := getPropertyFiles(files, bucket, svc); err != nil {
		return err
	}

	return nil
}

func getPropertyFiles(files []string, b string, svc s3iface.S3API) error {
	services := make(map[string][]byte)
	globals := make(map[int][]byte)

	for i, f := range files {
		body, isService, _ := getFile(f, b, svc)
		if isService {
			pathSplit := strings.Split(f, "/")
			service := pathSplit[len(pathSplit)-1]
			serviceName := service[0 : len(service)-5]
			services[serviceName] = body
		} else {
			globals[i] = body
		}
		i++
	}

	s, err := mergeAll(globals, services)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	for k, v := range s {
		kv.WriteProperty(k, v)
	}

	return nil
}

func mergeAll(globals map[int][]byte, services map[string][]byte) (map[string][]byte, error) {
	var m1, m2, m3 map[string]interface{}
	var lastMerged map[string]interface{}

	for i := 0; i < len(globals); i++ {
		if lastMerged != nil {
			m1 = lastMerged
		} else {
			if err := json.Unmarshal(globals[i], &m1); err != nil {
				return nil, err
			}
		}

		if globals[i+1] == nil {
		} else {
			if err := json.Unmarshal(globals[i+1], &m2); err != nil {
				return nil, err
			}
			mergemap.Merge(m1, m2)

			lastMerged = m1
		}
	}

	mergedServices := make(map[string][]byte)
	for k, s := range services {
		if err := json.Unmarshal(s, &m3); err != nil {
			return nil, err
		}
		mergemap.Merge(m3, m1)
		// TODO: append instance metadata
		finalBytes, _ := json.Marshal(m3)
		mergedServices[k] = finalBytes
	}

	return mergedServices, nil
}

func getFile(k, b string, svc s3iface.S3API) ([]byte, bool, error) {

	var body []byte

	if isJSON.MatchString(k) {
		result, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(b),
			Key:    aws.String(k),
		})

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
				log.Errorf("Download canceled due to timeout %v\n", err)
				Health = false
				return nil, false, err
			} else {
				log.Errorf("Failed to download object: %v", err)
				Health = false
				return nil, false, err
			}
		}

		body, err = ioutil.ReadAll(result.Body)
		if err != nil {
			log.Errorf("Failure to read body: %v\n", err)
			Health = false
			return nil, false, err
		}
	} else {
		log.Printf("Skipping: %v.\n", k)
	}

	var isService bool

	if strings.Contains(k, "service") {
		isService = true
	} else {
		isService = false
	}

	return body, isService, nil
}
