package s3

import (
	"encoding/json"
	"fmt"
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
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"

	"github.com/rapid7/cps/index"
	"github.com/rapid7/cps/kv"
	"github.com/rapid7/cps/secret"
)

// SecretHandlerVersion is a type indicating which version of secret handler we should use
type SecretHandlerVersion int

const (
	// V0 is iota
	V0 SecretHandlerVersion = iota
	// V1 is the v1 version enum-ish value of secret injection
	V1
	// V2 is the v2 version enum-ish value of secret injection
	V2
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
	bucket               string
	bucketRegion         string
	secretHandlerVersion SecretHandlerVersion
}

// S3API is a local wrapper over aws-sdk-go's S3 API
type S3API interface { //nolint: golint
	s3iface.S3API
}

// Poll polls every 60 seconds, kicking off an S3 sync.
func Poll(bucket, bucketRegion string, v SecretHandlerVersion, log *zap.Logger) {
	Config = config{
		bucket:               bucket,
		bucketRegion:         bucketRegion,
		secretHandlerVersion: v,
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
		log.Error("failed to list bucket",
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

func setUpAwsSession(region string) S3API {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	var svc S3API = s3.New(sess)

	return svc
}

func listBucket(bucket, region string, svc S3API, log *zap.Logger) ([]*s3.ListObjectsOutput, error) {
	i, err := index.ParseIndex(bucket, region)
	if err != nil {
		return nil, err
	}

	log.Info("using index to map index.yml/json dynamic values",
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
			log.Error("error listing s3 objects",
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

func parseAllFiles(resp []*s3.ListObjectsOutput, bucket string, svc S3API, log *zap.Logger) error {
	var files []string

	for _, object := range resp {
		for _, key := range object.Contents {
			files = append(files, *key.Key)
		}
	}

	return getPropertyFiles(files, bucket, svc, log)
}

func getPropertyFiles(files []string, b string, svc S3API, log *zap.Logger) error {
	services := make(map[string]interface{})

	for _, f := range files {
		body, err := getFile(f, b, svc, log)
		if err != nil {
			log.Error("error getting file",
				zap.Error(err),
				zap.String("file", f),
			)
			return err
		}

		pathSplit := strings.Split(f, "/")
		service := pathSplit[len(pathSplit)-1]
		serviceName := service[0 : len(service)-5]
		serviceProperties := make(map[string]interface{})
		if err := json.Unmarshal(body, &serviceProperties); err != nil {
			log.Error("error unmarshalling properties",
				zap.Error(err),
				zap.String("service_name", serviceName),
				zap.String("file", f),
			)

			return err
		}

		log.Debug("parsed properties file",
			zap.String("service", serviceName),
			zap.String("file", f),
		)

		services[serviceName] = serviceProperties
	}

	var sm map[string]interface{}
	switch Config.secretHandlerVersion {
	case V1:
		var err error
		sm, err = injectSecrets(services)
		if err != nil {
			log.Error("error injecting secrets",
				zap.Error(err),
				zap.Any("services", services),
				zap.Any("inject_version", Config.secretHandlerVersion),
			)

			return err
		}
	case V2:
		s, err := injectSecretsV2(log, services)
		if err != nil {
			log.Error("error injecting secrets",
				zap.Error(err),
				zap.Any("services", services),
				zap.Any("inject_version", Config.secretHandlerVersion),
			)

			return err
		}
		var ok bool
		sm, ok = s.(map[string]interface{})
		if !ok {
			log.Error("error handling properties from secret injection",
				zap.Any("services", services),
				zap.Any("inject_version", Config.secretHandlerVersion),
			)

			return err
		}
	default:
		log.Error("attempted to use an unsupported handler version",
			zap.Any("inject_version", Config.secretHandlerVersion),
		)

		return fmt.Errorf("invalid secret handler version: %v", Config.secretHandlerVersion)

	}

	for k, v := range sm {
		serviceBytes, err := json.Marshal(v)
		if err != nil {
			log.Error("There was an error marshalling properties for storage",
				zap.Error(err),
				zap.Any("value", v),
			)

			return err
		}
		if err := kv.WriteProperty(k, serviceBytes); err != nil {
			log.Error("There was an error writing properties to kv store",
				zap.Error(err),
				zap.String("key", k),
			)

			return err
		}
	}

	return nil
}

var getSSMClient = secret.GetSSMSession

// injectSecretsV2 improves upon the V1 mechanism by removing the use of reflection and correctly covering
// nested map and array cases.
// NOTE: We currently don't have a way of handling the case of an array of SSM objects (i.e. [{"$ssm": {}}, {"$ssm": {}}])
// because there's nothing to identify the property name.
func injectSecretsV2(log *zap.Logger, data interface{}) (interface{}, error) {
	switch val := data.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{})
		if len(val) == 0 {
			return out, nil
		}
		for k, v := range val {
			var injected interface{}
			var err error
			if s, ok := v.(map[string]interface{}); ok {
				if _, ok := s[secret.SSMIdentifier]; ok {
					var ssm secret.SSM
					if err := mapstructure.Decode(s, &ssm); err != nil {
						log.Error("unable to decode SSM stanza to struct",
							zap.Error(err),
							zap.String("key", k),
						)
						continue
					}
					if ssm.SSM.Region == "" {
						log.Error(secret.ErrMissingRegion.Error(),
							zap.String("key", k),
						)
						continue
					}
					svc := getSSMClient(ssm.SSM.Region)
					decrypted, err := secret.GetSSMSecretWithLabels(svc, k, ssm)
					if err != nil {
						return nil, err
					}
					out[k] = decrypted
					continue
				}
			}
			injected, err = injectSecretsV2(log, v)
			if err != nil {
				return nil, err
			}
			out[k] = injected
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, 0)
		if len(val) == 0 {
			return out, nil
		}
		for _, v := range val {
			injected, err := injectSecretsV2(log, v)
			if err != nil {
				return nil, err
			}
			out = append(out, injected)
		}
		return out, nil
	default:
		return val, nil
	}
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
						if _, ok := d.MapIndex(k).Interface().(map[string]interface{})[secret.SSMIdentifier]; ok {
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

func getFile(k, b string, svc S3API, log *zap.Logger) ([]byte, error) {

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
