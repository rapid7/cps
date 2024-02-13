package secret

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"go.uber.org/zap"
)

var (
	log *zap.Logger
	// ErrSSMMissingRegion is a typed error if a SSM stanza is missing a region
	ErrSSMMissingRegion = errors.New("SSM credential is missing the region key")
)

func init() {
	log, _ = zap.NewProduction()
}

const (
	// SSMIdentifier is the magic string identifying an SSM secret stanza
	SSMIdentifier = "$ssm"
)

// SSM is a plain-old-Go-object for carrying structured SSM stanzas in CPS props
type SSM struct {
	SSM struct {
		Service   string `mapstructure:"service"`
		Region    string `mapstructure:"region"`
		Label     string `mapstructure:"label"`
		Encrypted string `mapstructure:"encrypted"`
	} `mapstructure:"$ssm"`
}

// SSMAPI is a local wrapper over aws-sdk-go's SSM API
type SSMAPI interface {
	ssmiface.SSMAPI
}

// GetSSMSecretWithLabels gets a decrypted SSM secret, supporting searching by labels as well
func GetSSMSecretWithLabels(ctx context.Context, svc SSMAPI, name string, cred SSM) (string, error) {
	if cred.SSM.Region == "" || cred.SSM.Encrypted == "" {
		return "", errors.New("not a valid SSM stanza")
	}
	path := "/"
	if cred.SSM.Service != "" {
		path += cred.SSM.Service+"/"
	}
	log.Info("debug - path name",
		zap.Any("path", path),
		zap.Any("name", name),
	)

	params := &ssm.GetParametersByPathInput{
		Path:           aws.String(path),
		WithDecryption: aws.Bool(true),
		MaxResults: 10,
	}
	log.Info("debug - params",
		zap.Any("params", params),
	)

	if cred.SSM.Label != "" {
		params.ParameterFilters = []*ssm.ParameterStringFilter{
			{
				Key:    aws.String("Label"),
				Option: aws.String("Equals"),
				Values: aws.StringSlice([]string{cred.SSM.Label}),
			},
		}
	}

	p, err := svc.GetParametersByPathWithContext(ctx, params)
	log.Info("debug - p",
		zap.Any("p", p),
	)
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
	for nt, next_token := range p.Parameters {
		for _, param := range p.Parameters {
			log.Info("debug - param",
				zap.Any("param ARN", aws.StringValue(param.ARN)),
				zap.Any("param Name", aws.StringValue(param.Name)),
			)
			parameterName := aws.StringValue(param.Name)
			if cred.SSM.Service != "" {
				if strings.Replace(parameterName, path, "", 1) == name {
					found = aws.StringValue(param.Value)
					log.Info("debug - found1",
						zap.Any("found", found),
					)
					break
				}
			} else {
				found = aws.StringValue(param.Value)
				log.Info("debug - found2",
					zap.Any("found", found),
				)
				break
			}
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

// GetSSMSession gets a regional SSM session
func GetSSMSession(region string) SSMAPI {
	var svc SSMAPI = ssm.New(getSession(region))
	return svc
}

// GetSSMSecret parses all properties looking for an
// $ssm key. When found, it gets the ssm parameter store
// secret and writes the key and secret to GetSSMSessionthe kv store.
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
