package secret

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func GetSSMSecret(k string, v []byte) (string, error) {
	var j map[string]interface{}
	err := json.Unmarshal(v, &j)
	if err != nil {
		log.Errorf("Failed to unmarshall SSM object: %v", err)
		return "", err
	}

	var region string
	var service string
	if _, ok := j["$ssm"].(map[string]interface{})["service"]; ok {
		data := j["$ssm"].(map[string]interface{})
		service = data["service"].(string)
		region = data["region"].(string)
		k = "/" + service + "/" + k

		sess := session.Must(session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				Region: aws.String(region),
			},
		}))

		svc := ssm.New(sess)

		decrypt := true
		params := &ssm.GetParameterInput{
			Name:           &k,
			WithDecryption: &decrypt,
		}

		p, err := svc.GetParameter(params)
		if err != nil {
			log.Errorf("Error getting SSM parameter %v: %v", k, err)
			return "", err
		}

		return *p.Parameter.Value, nil
	}

	if _, ok := j["$ssm"]; ok {
		data := j["$ssm"].(map[string]interface{})
		region = data["region"].(string)
	} else {
		return "", errors.New("Object is not an SSM stanza")
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	svc := ssm.New(sess)

	decrypt := true
	params := &ssm.GetParameterInput{
		Name:           &k,
		WithDecryption: &decrypt,
	}

	p, err := svc.GetParameter(params)
	if err != nil {
		log.Errorf("Error getting SSM parameter %v: %v", k, err)
		return "", err
	}

	return *p.Parameter.Value, nil
}
