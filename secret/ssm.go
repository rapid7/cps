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

const (
	SSMIdentifier = "$ssm"
)

type SSM struct {
	SSM struct{
		Service string `mapstructure:"service"`
		Region string `mapstructure:"region"`
		Label string `mapstructure:"label"`
		Encrypted string `mapstructure:"encrypted"`
	} `mapstructure:"$ssm"`
}

func GetSSMSecretWithLabels(name string, cred SSM) (string, error) {
	if cred.SSM.Region == "" || cred.SSM.Encrypted == "" {
		return "", errors.New("not a valid SSM stanza")
	}
	path := "/"
	if cred.SSM.Service != "" {
		path += cred.SSM.Service
	}
	
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(cred.SSM.Region),
		},
	}))
	svc := ssm.New(sess)
	params := &ssm.GetParametersByPathInput{
		Path:             aws.String(path),
		WithDecryption:   aws.Bool(true),
	}

	if cred.SSM.Label != "" {
		params.ParameterFilters = []*ssm.ParameterStringFilter{
			{
				Key:    aws.String("Label"),
				Option: aws.String("Equals"),
				Values: aws.StringSlice([]string{cred.SSM.Label}),
			},
		}
	}


	p, err := svc.GetParametersByPath(params)
	if err != nil {
		log.Error("Error getting SSM parameters",
			zap.Error(err),
			zap.String("path", path),
			zap.String("label", cred.SSM.Label),
			zap.String("key", name),
		)

		return "", err
	}

	var found string
	for _, param := range p.Parameters {
		if aws.StringValue(param.Name) == name {
			found = aws.StringValue(param.Value)
			break
		}
	}

	if found == "" {
		err := errors.New("no matching parameter found")
		log.Error("Error getting SSM parameter",
			zap.Error(err),
			zap.String("path", path),
			zap.String("label", cred.SSM.Label),
			zap.String("key", name),
		)

		return "", err
	}

	return found, nil
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

	if j[SSMIdentifier] == nil {
		return "", errors.New("$ssm is nil, this is most likely due to an indentation problem")
	}

	var region string
	var service string
	if _, ok := j[SSMIdentifier].(map[string]interface{})["service"]; ok {
		data := j[SSMIdentifier].(map[string]interface{})
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

	if _, ok := j[SSMIdentifier]; ok {
		data := j[SSMIdentifier].(map[string]interface{})
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
