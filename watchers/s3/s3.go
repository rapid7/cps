package s3

import (
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/buger/jsonparser"

	log "github.com/sirupsen/logrus"

	kv "cps/pkg/kv"
	secret "cps/pkg/secret"
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
	Health = false
	Up = false

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.Print("connecting to s3...")
	// log.SetLevel(log.DebugLevel)
}

func Poll(bucket, bucketRegion string) {
	Config = config{
		bucket:       bucket,
		bucketRegion: bucketRegion,
	}

	Sync(time.Now())
	doEvery(60*time.Second, Sync)
}

func Sync(t time.Time) {
	log.Print("s3 sync begun")

	bucket := Config.bucket
	region := Config.bucketRegion

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))
	svc := s3.New(sess)

	params := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	}

	resp, err := svc.ListObjects(params)
	if err != nil {
		log.Errorf("Error listing s3 objects %v:", err)
		Health = false
		return
	}

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

	Up = true
	Health = true
}

func parsePropertyFile(k string, b string, svc *s3.S3) {
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
		properties := make(map[string]interface{})

		jsonparser.ObjectEach(body, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch dataTypeString := dataType.String(); dataTypeString {
			case "string":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				properties[string(key)] = string(value)
			case "number":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				v, _ := strconv.Atoi(string(value))
				properties[string(key)] = v
			case "boolean":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				v, _ := strconv.ParseBool(string(value))
				properties[string(key)] = v
			case "null":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				properties[string(key)] = ""
			case "object":
				// TODO: Decrypt secret here.
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				secret.Decrypt(value)
			default:
				log.Errorf("Service: %v | Key: %v | Value %v | Type: %v | Unsupported! %v:%T", k, string(key), string(value), dataTypeString, dataTypeString, dataTypeString)
			}

			return nil
		}, "properties")

		kv.WriteProperty(path, properties)

	} else {
		log.Printf("Skipping: %v.\n", k)
	}
}

func doEvery(d time.Duration, f func(time.Time)) {
	for x := range time.Tick(d) {
		f(x)
	}
}
