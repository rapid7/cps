package s3

import (
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/buger/jsonparser"

	log "github.com/sirupsen/logrus"

	"github.com/rapid7/cps/pkg/kv"
	"github.com/rapid7/cps/pkg/secret"
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
	// log.SetLevel(log.DebugLevel)
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
		properties := make(map[string]interface{})

		jsonparser.ObjectEach(body, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch dataTypeString := dataType.String(); dataTypeString {
			case "string":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				properties[string(key)] = string(value)
			case "number":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				var v interface{}
				if strings.Contains(string(value), ".") {
					v, _ = strconv.ParseFloat(string(value), 64)
				} else {
					v, _ = strconv.Atoi(string(value))
				}
				properties[string(key)] = v
			case "boolean":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				v, _ := strconv.ParseBool(string(value))
				properties[string(key)] = v
			case "null":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				properties[string(key)] = ""
			case "object":
				log.Debugf("Wrote %s/%s:(%s)=%s", path, string(key), dataTypeString, string(value))
				s, err := secret.GetSSMSecret(string(key), value)
				if err != nil {
					handleSecretFailure(err, properties, string(key), path)
				} else {
					properties[string(key)] = s
				}
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

func handleSecretFailure(err error, properties map[string]interface{}, key, path string) {
	if err.Error() != "Object is not an SSM stanza" {
		k := kv.GetProperty(path)
		if k != nil {
			v := k.(map[string]interface{})
			if v[string(key)] != nil {
				sv := v[string(key)].(string)
				properties[string(key)] = sv
			}
		}
	}
}
