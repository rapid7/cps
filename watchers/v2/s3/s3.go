package s3

import (
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
	// TODO: just make this an interface{}
	services := make(map[string][]byte)

	for i, f := range files {
		// TODO: When this stabilizes, remove isService from getFile,
		// it is always true.
		body, isService, _ := getFile(f, b, svc)
		if isService {
			pathSplit := strings.Split(f, "/")
			service := pathSplit[len(pathSplit)-1]
			serviceName := service[0 : len(service)-5]
			services[serviceName] = body
		}
		i++
	}

	s, err := injectSecrets(services)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	for k, v := range s {
		kv.WriteProperty(k, v)
	}

	return nil
}

func injectSecrets(data interface{}) (map[string]interface{}, error) {
	d := reflect.ValueOf(data)
	tmpData := make(map[string]interface{})

	log.Info(kv.Cache)

	for _, k := range d.MapKeys() {
		match, _ := regexp.MatchString("$ssm", k.String())
		typeOfValue := reflect.TypeOf(d.MapIndex(k).Interface()).Kind()
		if match {
			key := k.String()
			secret, err := secret.GetSSMSecret(key, d.MapIndex(k).Bytes())
			if err != nil {
				// TODO: final argument is the path in kv.Store
				return nil, err
				// handleSecretFailure(err, properties, key, "")
			}
			tmpData[key] = secret
		} else {
			key := k.String()
			if typeOfValue == reflect.Map {
				iter, _ := injectSecrets(d.MapIndex(k).Interface)
				tmpData[key] = iter
			} else {
				tmpData[key] = d.MapIndex(k).Interface()
			}
		}
	}

	return tmpData, nil
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
