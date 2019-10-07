package secret

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"go.uber.org/zap"
)

var (
	log *zap.Logger
)

func init() {
	log, _ = zap.NewProduction()
}

// GetSSMSecret parses all properties looking for an
// $ssm key. When found, it gets the ssm parameter store
// secret and writes the key and secret to the kv store.
func GetSSMSecret(k string, v []byte) (string, error) {
	var j map[string]interface{}
	err := json.Unmarshal(v, &j)
	if err != nil {
		log.Error("Failed to unmarshall SSM object",
			zap.Error(err),
		)

		return "", err
	}

	if j["$ssm"] == nil {
		return "", errors.New("$ssm is nil, this is most likely due to an indentation problem")
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
			log.Error("Error getting SSM parameter",
				zap.Error(err),
				zap.String("key", k),
			)

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
		log.Error("Error getting SSM parameter",
			zap.Error(err),
			zap.String("key", k),
		)

		return "", err
	}

	return *p.Parameter.Value, nil
}
