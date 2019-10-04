package index

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	"github.com/rapid7/cps/pkg/ec2meta"
)

var (
	metadata ec2meta.Instance
)

// Source locations (s3, file, consul, etc).
type Source struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Parameters struct {
		Path string `yaml:"path"`
	} `yaml:"parameters"`
}

// Index is the top level struct which the index is mapped to.
type Index struct {
	Version float64  `yaml:"version"`
	Sources []Source `yaml:"sources"`
}

// ParseIndex grabs the index from s3 and returns all file paths.
func ParseIndex(b, region string) ([]string, error) {
	jsonBytes, err := getIndexFromS3(b, region)
	if err != nil {
		return nil, err
	}

	var index Index

	if err := json.Unmarshal(jsonBytes, &index); err != nil {
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
		return nil, err
	}

	defer result.Body.Close()

	metadata = ec2meta.Populate(sess)

	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
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
					injectedPath.WriteString(metadata.Account + ".json")
				} else {
					injectedPath.WriteString(metadata.Account + "/")
				}
			case strings.Contains(p, "instance:vpc"):
				if i == (len(split) - 1) {
					injectedPath.WriteString(metadata.VpcID + ".json")
				} else {
					injectedPath.WriteString(metadata.VpcID + "/")
				}
			case strings.Contains(p, "instance:region"):
				if i == (len(split) - 1) {
					injectedPath.WriteString(metadata.Region + ".json")
				} else {
					injectedPath.WriteString(metadata.Region + "/")
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
