package s3

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
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
	"go.uber.org/zap"

	"github.com/rapid7/cps/index"
	"github.com/rapid7/cps/kv"
	"github.com/rapid7/cps/secret"
)

var (
	// Up contains the systems availability. If true there are no issues with s3.
	Up bool

	// Health contains the system's readiness. If false the watcher
	// could not list objects. There are still probably objects in the kv
	// store so the service is still considered "Up".
	Health bool

	// Config exports the config struct. Need to make export
	// the config struct itself (TODO).
	Config config
	isJSON = regexp.MustCompile(".json$")
	mu     = sync.Mutex{}
)

type config struct {
	bucket       string
	bucketRegion string
}

// Poll polls every 60 seconds, kicking off an S3 sync.
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

// Sync is the main function for the s3 watcher. It sets up the
// AWS session, lists all items in the bucket, finally
// parsing all files and putting them in the kv store.
func Sync(t time.Time, log *zap.Logger) {
	log.Info("S3 sync begun")

	bucket := Config.bucket
	region := Config.bucketRegion

	svc := setUpAwsSession(region)
	resp, err := listBucket(bucket, region, svc, log)
	if err != nil {
		log.Error("Failed to list bucket",
			zap.Error(err),
			zap.String("bucket", bucket),
			zap.String("region", region),
		)

		return
	}

	if err := parseAllFiles(resp, bucket, svc, log); err != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()
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

func listBucket(bucket, region string, svc s3iface.S3API, log *zap.Logger) ([]*s3.ListObjectsOutput, error) {
	i, err := index.ParseIndex(bucket, region)
	if err != nil {
		return nil, err
	}

	log.Info("Using index to map index.yml/json dynamic values",
		zap.Any("index", i),
	)

	var responses []*s3.ListObjectsOutput

	for _, prefix := range i {
		params := &s3.ListObjectsInput{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		}

		resp, err := svc.ListObjects(params)
		if err != nil {
			log.Error("Error listing s3 objects",
				zap.Error(err),
				zap.String("bucket", bucket),
				zap.String("region", region),
				zap.String("prefix", prefix),
			)

			Health = false

			return nil, err
		}

		responses = append(responses, resp)
	}

	return responses, nil
}

func parseAllFiles(resp []*s3.ListObjectsOutput, bucket string, svc s3iface.S3API, log *zap.Logger) error {
	var files []string

	for _, object := range resp {
		for _, key := range object.Contents {
			files = append(files, *key.Key)
		}
	}

	if err := getPropertyFiles(files, bucket, svc, log); err != nil {
		return err
	}

	return nil
}

func getPropertyFiles(files []string, b string, svc s3iface.S3API, log *zap.Logger) error {
	services := make(map[string]interface{})

	for _, f := range files {
		body, _ := getFile(f, b, svc, log)
		pathSplit := strings.Split(f, "/")
		service := pathSplit[len(pathSplit)-1]
		serviceName := service[0 : len(service)-5]
		serviceProperties := make(map[string]interface{})
		err := json.Unmarshal(body, &serviceProperties)
		if err != nil {
			log.Error("There was an error unmarshalling properties",
				zap.Error(err),
				zap.String("service_name", serviceName),
				zap.String("file", f),
			)

			return err
		}

		services[serviceName] = serviceProperties
	}

	s, err := injectSecrets(services)
	if err != nil {
		log.Error("There was an error injecting secrets",
			zap.Error(err),
			zap.Any("services", services),
		)

		return err
	}

	for k, v := range s {
		serviceBytes, _ := json.Marshal(v)
		kv.WriteProperty(k, serviceBytes)
	}

	return nil
}

func injectSecrets(data interface{}) (map[string]interface{}, error) {
	d := reflect.ValueOf(data)

	td := make(map[string]interface{})
	for _, k := range d.MapKeys() {
		if reflect.ValueOf(d.MapIndex(k).Interface()).Kind() == reflect.Map {
			di := reflect.ValueOf(d.MapIndex(k).Interface())

			for _, ik := range di.MapKeys() {
				if _, ok := di.MapIndex(ik).Interface().(map[string]interface{}); ok {
					valueT := reflect.TypeOf(di.MapIndex(ik).Interface()).Kind()
					if valueT == reflect.Map {
						// This is an ssm object. Get The secret's value
						// and add it to the map we return.
						if _, ok := d.MapIndex(k).Interface().(map[string]interface{})["$ssm"]; ok {
							secretBytes, _ := json.Marshal(d.MapIndex(k).Interface())
							s, err := secret.GetSSMSecret(k.String(), secretBytes)
							if err != nil {
								return nil, err
							}

							if td[k.String()] == nil {
								td[k.String()] = make(map[string]interface{})
							}

							td[k.String()] = s
							td, _ = injectSecrets(td)
						} else {
							// This is not an ssm object, but is an object.
							// Add it to the map we return.
							if td[k.String()] == nil {
								td[k.String()] = make(map[string]interface{})
							}

							keyMap := td[k.String()].(map[string]interface{})
							if valueT == reflect.Map || valueT == reflect.Slice {
								keyMap[ik.String()], _ = injectSecrets(di.MapIndex(ik).Interface())
							} else {
								keyMap[ik.String()] = di.MapIndex(ik).Interface()
							}
						}
					} else {
						// This is not a map. Add the value to the inner key.
						if td[k.String()] == nil {
							td[k.String()] = make(map[string]interface{})
						}

						keyMap := td[k.String()].(map[string]interface{})
						keyMap[ik.String()] = di.MapIndex(ik).Interface()
					}
				}
			}
		} else {
			// Not a map, this is a top level property. Process
			// accordingly.
			td[k.String()] = d.MapIndex(k).Interface()
		}
	}

	return td, nil
}

func getFile(k, b string, svc s3iface.S3API, log *zap.Logger) ([]byte, error) {

	var body []byte

	if isJSON.MatchString(k) {
		result, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(b),
			Key:    aws.String(k),
		})

		if err != nil {
			if aerr, ok := err.(awserr.Error); ok && aerr.Code() == request.CanceledErrorCode {
				log.Error("Download canceled due to timeout",
					zap.Error(err),
					zap.String("key", k),
					zap.String("bucket", b),
				)

				Health = false

				return nil, err
			}

			log.Error("Failed to download object",
				zap.Error(err),
				zap.String("key", k),
				zap.String("bucket", b),
			)

			Health = false

			return nil, err
		}

		body, err = ioutil.ReadAll(result.Body)
		defer result.Body.Close()
		if err != nil {
			log.Error("Failure to read body:",
				zap.Error(err),
				zap.String("key", k),
				zap.String("bucket", b),
			)

			Health = false

			return nil, err
		}
	} else {
		log.Info("Skipping key",
			zap.String("key", k),
		)
	}

	return body, nil
}
