package s3

import (
	"encoding/json"
	"io/ioutil"
	"os"
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

	log "github.com/sirupsen/logrus"

	"github.com/rapid7/cps/pkg/index"
	"github.com/rapid7/cps/pkg/kv"
	"github.com/rapid7/cps/pkg/secret"
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
	services := make(map[string]interface{})

	for i, f := range files {
		// TODO: When this stabilizes, remove isService from getFile,
		// it is always true.
		body, isService, _ := getFile(f, b, svc)
		if isService {
			pathSplit := strings.Split(f, "/")
			service := pathSplit[len(pathSplit)-1]
			serviceName := service[0 : len(service)-5]
			serviceProperties := make(map[string]interface{})
			err := json.Unmarshal(body, &serviceProperties)
			if err != nil {
				log.Errorf("There was an error unmarshalling properties for %v: %v", serviceName, err)
				return err
			}
			services[serviceName] = serviceProperties
		}
		i++
	}

	s, err := injectSecrets(services)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	log.Debugf("All services found: %v", s)
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
				valueT := reflect.TypeOf(di.MapIndex(ik).Interface()).Kind()
				if valueT == reflect.Map {
					// This is an ssm object. Get The secret's value
					// and add it to the map we return.
					if _, ok := d.MapIndex(k).Interface().(map[string]interface{})["$ssm"]; ok {
						secretBytes, _ := json.Marshal(d.MapIndex(k).Interface())
						s, err := secret.GetSSMSecret(k.String(), secretBytes)
						if err != nil {
							log.Error(err)
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

						if valueT == reflect.Map || valueT == reflect.Slice {
							td[k.String()].(map[string]interface{})[ik.String()], _ = injectSecrets(di.MapIndex(ik).Interface())
						} else {
							td[k.String()].(map[string]interface{})[ik.String()] = di.MapIndex(ik).Interface()
						}
					}
				} else {
					// This is not a map. Add the value to the inner key.
					if td[k.String()] == nil {
						td[k.String()] = make(map[string]interface{})
					}

					td[k.String()].(map[string]interface{})[ik.String()] = di.MapIndex(ik).Interface()
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
		defer result.Body.Close()
		if err != nil {
			log.Errorf("Failure to read body: %v\n", err)
			Health = false
			return nil, false, err
		}
	} else {
		log.Printf("Skipping: %v.\n", k)
	}

	// We are moving toward a new directory structure without
	// `service` in the path.
	isService := true

	return body, isService, nil
}
