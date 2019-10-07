package s3

import (
	"io/ioutil"
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
	"go.uber.org/zap"

	"github.com/rapid7/cps/kv"
	"github.com/rapid7/cps/secret"
)

var (
	// Up contains the systems availability. If true the s3 service is up.
	Up bool

	// Health contains the systems readiness. If false the s3 service
	// most likely can't read or download from s3. Most likely temporary.
	Health bool

	// Config contains parameters related to S3.
	Config config
)

type config struct {
	bucket       string
	bucketRegion string
}

func init() {
	Health = false
	Up = false
}

// Poll polls every 60 seconds, kicking of an s3 sync.
func Poll(bucket, bucketRegion string, log *zap.Logger) {
	Config = config{
		bucket:       bucket,
		bucketRegion: bucketRegion,
	}

	Sync(time.Now(), log)

	ticker := time.NewTicker(60 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				Sync(time.Now(), log)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

// Sync sets up an s3 session, parses all files and puts
// them into the kv store.
func Sync(t time.Time, log *zap.Logger) {
	log.Info("S3 sync begun")

	bucket := Config.bucket
	region := Config.bucketRegion

	svc := setUpAwsSession(region)
	resp, err := listBucket(bucket, svc, log)
	if err != nil {
		return
	}

	err = parseAllFiles(resp, bucket, svc, log)
	if err != nil {
		return
	}

	Up = true
	Health = true

	log.Info("S3 sync finished")
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

func listBucket(bucket string, svc s3iface.S3API, log *zap.Logger) (*s3.ListObjectsOutput, error) {
	params := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	}

	resp, err := svc.ListObjects(params)
	if err != nil {
		log.Error("Error listing s3 objects",
			zap.Error(err),
		)

		Health = false

		return nil, err
	}

	return resp, nil
}

func parseAllFiles(resp *s3.ListObjectsOutput, bucket string, svc s3iface.S3API, log *zap.Logger) error {
	var wg sync.WaitGroup
	wg.Add(len(resp.Contents))

	numCores := runtime.NumCPU()
	guard := make(chan struct{}, numCores*32)

	for _, key := range resp.Contents {
		guard <- struct{}{}
		go func(key *s3.Object) {
			defer wg.Done()
			parsePropertyFile(*key.Key, bucket, svc, log)
			<-guard
		}(key)
	}

	wg.Wait()

	return nil
}

func parsePropertyFile(k string, b string, svc s3iface.S3API, log *zap.Logger) {
	isJSON, _ := regexp.Compile(".json$")

	if isJSON.MatchString(k) {
		result, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(b),
			Key:    aws.String(k),
		})

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
				log.Error("Download canceled due to timeout",
					zap.Error(err),
				)

				Health = false

				return
			}

			log.Error("Failed to download object",
				zap.Error(err),
				zap.String("bucket", b),
				zap.String("object", k),
			)

			Health = false

			return
		}

		body, err := ioutil.ReadAll(result.Body)
		if err != nil {
			log.Error("Failed to read body",
				zap.Error(err),
				zap.String("bucket", b),
				zap.String("object", k),
			)

			Health = false

			return
		}

		// Removes .json extension.
		path := k[0 : len(k)-5]
		properties := make(map[string]interface{})

		jsonparser.ObjectEach(body, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch dataTypeString := dataType.String(); dataTypeString {
			case "string":
				properties[string(key)] = string(value)
			case "number":
				var v interface{}
				if strings.Contains(string(value), ".") {
					v, _ = strconv.ParseFloat(string(value), 64)
				} else {
					v, _ = strconv.Atoi(string(value))
				}
				properties[string(key)] = v
			case "boolean":
				v, _ := strconv.ParseBool(string(value))
				properties[string(key)] = v
			case "null":
				properties[string(key)] = ""
			case "object":
				s, err := secret.GetSSMSecret(string(key), value)
				if err != nil {
					handleSecretFailure(err, properties, string(key), path)
				} else {
					properties[string(key)] = s
				}
			default:
				log.Error("Unsupported type!",
					zap.String("key", string(key)),
					zap.String("value", string(value)),
					zap.String("type", dataTypeString),
				)
			}

			return nil
		}, "properties")

		kv.WriteProperty(path, properties)

	} else {
		log.Info("Skipping file without json extension",
			zap.String("file", k),
		)
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
