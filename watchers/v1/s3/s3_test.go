package s3

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/rapid7/cps/watchers/v1/s3/mocks"
)

func TestListBucket(t *testing.T) {
	svc := new(mocks.S3API)
	svc.On("ListObjects", mock.AnythingOfType("*s3.ListObjectsInput")).Return(&s3.ListObjectsOutput{
		Contents: []*s3.Object{
			&s3.Object{Key: aws.String("1234567890/us-east-1/service-one.json")},
			&s3.Object{Key: aws.String("1234567890/us-east-1/service-two.json")},
		},
	}, nil)

	b, err := listBucket("test.bucket", svc)
	assert.Nil(t, err, "Expected no error")
	assert.Len(t, b.Contents, 2, "Expected two keys")
	assert.Equal(t, aws.String("1234567890/us-east-1/service-one.json"), b.Contents[0].Key, "Expected service-one")
	assert.Equal(t, aws.String("1234567890/us-east-1/service-two.json"), b.Contents[1].Key, "Expected service-two")
}

func TestParseAllFiles(t *testing.T) {
	svc := new(mocks.S3API)

	o := &s3.ListObjectsOutput{
		Contents: []*s3.Object{
			&s3.Object{Key: aws.String("1234567890/us-east-1/service-one.json")},
			&s3.Object{Key: aws.String("1234567890/us-east-1/service-two.json")},
			&s3.Object{Key: aws.String("1234567890/us-east-1/.not-a-service-file")},
		},
	}

	body := ioutil.NopCloser(strings.NewReader(`{
			"property.one": true,
			"property.two": "foo",
			"property.three": 1
		}`))

	svc.On("GetObject", mock.AnythingOfType("*s3.GetObjectInput")).Return(&s3.GetObjectOutput{
		Body: body,
	}, nil)

	err := parseAllFiles(o, "test.bucket", svc)

	assert.Nil(t, err, "Expected no error")
}
