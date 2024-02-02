package index

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"

	"github.com/rapid7/cps/ec2meta"
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
    accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
    secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
    creds := credentials.NewStaticCredentials(accessKey, secretKey, os.Getenv("AWS_SESSION_TOKEN"))
	sess, err := session.NewSession(&aws.Config{
        Credentials: creds,
        Region:      aws.String(region),
    })
    if err != nil {
        panic(err)
    }

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
