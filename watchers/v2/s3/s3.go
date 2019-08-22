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

	log.Infof("SERVICES %v:", s)
	for k, v := range s {
		log.Infof("writing %v to %v", k, v)
		serviceBytes, _ := json.Marshal(v)
		kv.WriteProperty(k, serviceBytes)
	}

	log.Info(kv.Cache)

	return nil
}

func injectSecrets(data interface{}) (map[string]interface{}, error) {
	finalData := make(map[string]interface{})
	d := reflect.ValueOf(data)

	// asset-ui:map
	tmpData := make(map[string]interface{})
	for _, k := range d.MapKeys() {
		log.Info("OUTER KEY: %v", k)

		if reflect.ValueOf(d.MapIndex(k).Interface()).Kind() == reflect.Map {
			di := reflect.ValueOf(d.MapIndex(k).Interface())

			for _, ik := range di.MapKeys() {
				log.Infof("INNER KEY %v IS A %t", ik, ik)
				typeOfValue := reflect.TypeOf(di.MapIndex(ik).Interface()).Kind()
				if typeOfValue == reflect.Map {
					log.Infof("CPS THINKS %v is a MAP WITH VALUE %v", k.String(), d.MapIndex(k).Interface())
					// This is an ssm object, handle it.
					if _, ok := d.MapIndex(k).Interface().(map[string]interface{})["$ssm"]; ok {
						log.Infof("GET SECRET: %v", k.String())
						log.Infof("THE SECRET MMAP: %v", d.MapIndex(k).Interface())
						secretBytes, _ := json.Marshal(d.MapIndex(k).Interface())
						s, err := secret.GetSSMSecret(k.String(), secretBytes)
						if err != nil {
							log.Error(err)
							// TODO: final argument is the path in kv.Store
							return nil, err
							// handleSecretFailure(err, properties, key, "")
						}

						if tmpData[k.String()] == nil {
							tmpData[k.String()] = make(map[string]interface{})
						}

						// When ckrts process first, nothing else gets processed.
						log.Infof("ADDING CKRT TO MAP LIKE THIS: %v:%v:%t", k.String(), s, s)
						tmpData[k.String()] = s
						log.Infof("TMPDATA WHEN CKRT PROCESSES %v", tmpData)
						tmpData, _ = injectSecrets(tmpData)
					} else {
						log.Infof("CAUGHT STRANGE MAP %v:%v:%t", k.String(), di.MapIndex(ik).Interface())
						if tmpData[k.String()] == nil {
							tmpData[k.String()] = make(map[string]interface{})
						}

						log.Info("k.String: %v", k.String())
						if _, ok := tmpData[k.String()].(map[string]interface{})[ik.String()]; ok {
							// inner key exists
						} else {
							tmpData[k.String()].(map[string]interface{})[ik.String()] = make(map[string]interface{})
						}

						log.Info("debug")
						if typeOfValue == reflect.Map || typeOfValue == reflect.Slice {
							log.Infof("GOT TO A NESTED MAP OR SLICE %v:%v:%v:%t", k.String(), ik.String(), di.MapIndex(ik).Interface(), di.MapIndex(ik).Interface())
							tmpData[k.String()].(map[string]interface{})[ik.String()], _ = injectSecrets(di.MapIndex(ik).Interface())
							log.Infof("TMP DATA: %v", tmpData)
						} else {
							tmpData[k.String()].(map[string]interface{})[ik.String()] = di.MapIndex(ik).Interface()
							log.Infof("TMP DATA: %v", tmpData)
						}
					}
				} else {
					log.Infof("CATCH ALL IS WRITING %v TO %v[%v]", di.MapIndex(ik).Interface(), k.String(), ik.String())

					if tmpData[k.String()] == nil {
						tmpData[k.String()] = make(map[string]interface{})
					}

					tmpData[k.String()].(map[string]interface{})[ik.String()] = di.MapIndex(ik).Interface()
				}
			}
		} else {
			// Not a map, process accordingly.
			log.Infof("CATCHING STUFF IN THE ULTIMATE CATCH ALL %v:%v:%t", k.String(), d.MapIndex(k).Interface(), d.MapIndex(k).Interface())
			tmpData[k.String()] = d.MapIndex(k).Interface()
		}
	}
	finalData = tmpData
	return finalData, nil
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
