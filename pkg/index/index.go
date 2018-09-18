package index

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

type Source struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Parameters struct {
		Path string `yaml:"path"`
	} `yaml:"parameters"`
}

type Index struct {
	Version float64  `yaml:"version"`
	Sources []Source `yaml:"sources"`
}

func ParseIndex(b, region string) ([]string, error) {
	jsonBytes, err := getIndexFromS3(b, region)
	if err != nil {
		log.Errorf("Error getting index from S3: %v", err)
		return nil, err
	}

	var index Index

	err = json.Unmarshal(jsonBytes, &index)
	if err != nil {
		log.Errorf("Error parsing yaml: %v", err)
		return nil, err
	}

	var paths []string
	for _, p := range index.Sources {
		path := p.Parameters.Path
		if strings.Contains(path, "{{") {
			path = injectPath(path)
			paths = append(paths, path)
		} else {
			paths = append(paths, path)
		}
	}

	log.Println(paths)

	return paths, nil
}

func getIndexFromS3(b, region string) ([]byte, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	var svc s3iface.S3API = s3.New(sess)

	result, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(b),
		Key:    aws.String("index.json"),
	})

	if err != nil {
		log.Error(err)
		return nil, err
	}

	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
		log.Errorf("Failure to read body: %v\n", err)
		return nil, err
	}

	return body, nil
}

func injectPath(path string) string {
	var injectedPath bytes.Buffer

	split := strings.Split(path, "/")

	for i, p := range split {
		if strings.Contains(p, "{{") {
			switch {
			case strings.Contains(p, "instance:account"):
				if i == (len(split) - 1) {
					injectedPath.WriteString("00000000000.json")
				} else {
					injectedPath.WriteString("00000000000/")
				}
			case strings.Contains(p, "instance:vpc"):
				if i == (len(split) - 1) {
					injectedPath.WriteString("vpc-0000000.json")
				} else {
					injectedPath.WriteString("vpc-0000000/")
				}
			case strings.Contains(p, "instance:region"):
				if i == (len(split) - 1) {
					injectedPath.WriteString("us-east-1.json")
				} else {
					injectedPath.WriteString("us-east-1/")
				}
			default:
				injectedPath.WriteString("")
			}
		} else {
			injectedPath.WriteString(p + "/")
		}

	}

	return injectedPath.String()
}
