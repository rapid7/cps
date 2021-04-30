package s3

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/go-test/deep"
	"github.com/rapid7/cps/secret"
	"go.uber.org/zap"
)

var (
	service1Props = `{
	"properties": {
		"property.1": "delimited flat property",
		"property.2": {
			"value": "nested property"
		},
		"property.3": [
			1,
			2,
			3
		],
		"property.4": {
			"property.5": {
				"property.6": {
					"value": "deeply nested property"
				}
			}
		}
	}
}`
)

func TestNestedPropertiesAreConsistentAfterInjecting(t *testing.T) {
	log := zap.NewNop()

	var serviceProps map[string]interface{}
	if err := json.Unmarshal([]byte(service1Props), &serviceProps); err != nil {
		t.Fatal(err)
	}

	props := map[string]interface{}{
		"service1": serviceProps,
	}

	injectedProps, err := injectSecrets(props)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(props, injectedProps); diff == nil {
		t.Fatal("current implementation of `injectSecrets` is broken for nested properties. " +
			"If there's no diff, this test should be updated.")
	}

	recursiveInjectedProps, err := injectSecretsV2(log, props)
	if err != nil {
		t.Fatal(err)
	}

	if diff := deep.Equal(props, recursiveInjectedProps); diff != nil {
		t.Fatal(err)
	}
}

var (
	ErrNilValidator = errors.New("nil validator")
	ErrNilResponse  = errors.New("nil response")
)

type mockSSMService struct {
	secret.SSMAPI
	Validator func(input *ssm.GetParametersByPathInput) error
	Response  func() (*ssm.GetParametersByPathOutput, error)
}

func (m mockSSMService) GetParametersByPath(input *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
	if m.Validator == nil {
		return nil, ErrNilValidator
	}
	if err := m.Validator(input); err != nil {
		return nil, err
	}

	if m.Response == nil {
		return nil, ErrNilResponse
	}
	return m.Response()
}

func defaultValidator(input *ssm.GetParametersByPathInput) error {
	if *input.Path != "/" {
		return fmt.Errorf("no service key present, expected path to be `/` but got %s instead", *input.Path)
	}
	if input.ParameterFilters != nil {
		return fmt.Errorf("no label present, expected no parameter filters but got %v", input.ParameterFilters)
	}

	return nil
}

func TestInjectSecretsV2(t *testing.T) {
	log := zap.NewNop()

	testCases := []struct {
		name      string
		input     []byte
		validator func(input *ssm.GetParametersByPathInput) error
		output    func() (*ssm.GetParametersByPathOutput, error)
		expected  func(*testing.T, map[string]interface{}, interface{}, error)
	}{
		{
			name: "flat properties",
			input: []byte(`
{
	"properties": {
		"flat": {
			"$ssm": {
				"region": "us-east-1",
          		"encrypted": "ENCRYPTED SECRET"
			}
		}
	}
}`),
			validator: defaultValidator,
			output: func() (*ssm.GetParametersByPathOutput, error) {
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							DataType: aws.String("SecureString"),
							Name:     aws.String("flat"),
							Value:    aws.String("DECRYPTED!"),
						},
					},
				}, nil
			},
			expected: func(t *testing.T, props map[string]interface{}, newProps interface{}, err error) {
				if err != nil {
					t.Fatal(err)
				}

				diff := deep.Equal(props, newProps)
				if diff == nil {
					t.Fatal(diff)
				}

				if len(diff) != 1 {
					t.Fatalf("expected 1 diff but got %d", len(diff))
				}

				flatProp, err := nestedMapLookup(newProps.(map[string]interface{}), "service1", "properties", "flat")
				if err != nil {
					t.Fatal(err)
				}

				if flatProp.(string) != "DECRYPTED!" {
					t.Fatalf("expected `flat` property to be `DECRYPTED!` but got %s instead", flatProp)
				}
			},
		},
		{
			name: "nested properties",
			input: []byte(`
{
	"properties": {
		"nested": {
			"a": {
				"few": {
					"levels": {
						"$ssm": {
							"region": "us-east-1",
							"encrypted": "ENCRYPTED SECRET"
						}
					}
				}
			}
		}
	}
}`),
			validator: defaultValidator,
			output: func() (*ssm.GetParametersByPathOutput, error) {
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							DataType: aws.String("SecureString"),
							Name:     aws.String("levels"),
							Value:    aws.String("DECRYPTED!"),
						},
					},
				}, nil
			},
			expected: func(t *testing.T, props map[string]interface{}, newProps interface{}, err error) {
				if err != nil {
					t.Fatal(err)
				}

				diff := deep.Equal(props, newProps)
				if diff == nil {
					t.Fatal(diff)
				}

				if len(diff) != 1 {
					t.Fatalf("expected 1 diff but got %d", len(diff))
				}

				levelsProp, err := nestedMapLookup(newProps.(map[string]interface{}),
					"service1", "properties", "nested", "a", "few", "levels")
				if err != nil {
					t.Fatal(err)
				}

				if levelsProp.(string) != "DECRYPTED!" {
					t.Fatalf("expected `levelsProp` property to be `DECRYPTED!` but got %s instead", levelsProp)
				}
			},
		},
		{
			name: "multiple props",
			input: []byte(`
{
	"properties": {
		"nested": {
			"prop": {
				"$ssm": {
					"region": "us-east-1",
					"encrypted": "ENCRYPTED SECRET"
				}
			}
		},
		"prop": {
			"$ssm": {
				"region": "us-east-1",
				"encrypted": "ENCRYPTED SECRET"
			}
		}
	}
}`),
			validator: defaultValidator,
			output: func() (*ssm.GetParametersByPathOutput, error) {
				return &ssm.GetParametersByPathOutput{
					Parameters: []*ssm.Parameter{
						{
							DataType: aws.String("SecureString"),
							Name:     aws.String("prop"),
							Value:    aws.String("DECRYPTED!"),
						},
					},
				}, nil
			},
			expected: func(t *testing.T, props map[string]interface{}, newProps interface{}, err error) {
				if err != nil {
					t.Fatal(err)
				}

				diff := deep.Equal(props, newProps)
				if diff == nil {
					t.Fatal(diff)
				}

				if len(diff) != 2 {
					t.Fatalf("expected 2 diff but got %d", len(diff))
				}

				prop1, err := nestedMapLookup(newProps.(map[string]interface{}),
					"service1", "properties", "nested", "prop")
				if err != nil {
					t.Fatal(err)
				}

				prop2, err := nestedMapLookup(newProps.(map[string]interface{}),
					"service1", "properties", "prop")
				if err != nil {
					t.Fatal(err)
				}

				if prop1.(string) != "DECRYPTED!" {
					t.Fatalf("expected `prop1` property to be `DECRYPTED!` but got %s instead", prop1)
				}

				if prop2.(string) != "DECRYPTED!" {
					t.Fatalf("expected `prop2` property to be `DECRYPTED!` but got %s instead", prop2)
				}
			},
		},
		{
			name: "null values",
			input: []byte(`
{
	"properties": {
		"nested": {
			"prop": "hi!"
		},
		"prop": null
	}
}`),
			expected: func(t *testing.T, props map[string]interface{}, newProps interface{}, err error) {
				if err != nil {
					t.Fatal(err)
				}
				diff := deep.Equal(props, newProps)
				if diff != nil {
					t.Fatal(diff)
				}

				prop1, err := nestedMapLookup(newProps.(map[string]interface{}),
					"service1", "properties", "nested", "prop")
				if err != nil {
					t.Fatal(err)
				}

				prop2, err := nestedMapLookup(newProps.(map[string]interface{}),
					"service1", "properties", "prop")
				if err != nil {
					t.Fatal(err)
				}

				if prop1.(string) != "hi!" {
					t.Fatalf("expected `prop1` property to be `hi!` but got %s instead", prop1)
				}

				if prop2 != nil {
					t.Fatalf("expected `prop2` property to be nil but got %v instead", prop2)
				}
			},
		},
		{
			name: "empty array and object values",
			input: []byte(`
{
	"properties": {
		"nested": {
			"prop": {}
		},
		"prop": []
	}
}`),
			expected: func(t *testing.T, props map[string]interface{}, newProps interface{}, err error) {
				if err != nil {
					t.Fatal(err)
				}
				diff := deep.Equal(props, newProps)
				if diff != nil {
					t.Fatal(diff)
				}

				prop1, err := nestedMapLookup(newProps.(map[string]interface{}),
					"service1", "properties", "nested", "prop")
				if err != nil {
					t.Fatal(err)
				}

				prop2, err := nestedMapLookup(newProps.(map[string]interface{}),
					"service1", "properties", "prop")
				if err != nil {
					t.Fatal(err)
				}

				if m, ok := prop1.(map[string]interface{}); !ok || len(m) != 0 {
					t.Fatalf("expected `prop1` property to be an empty map but got %v instead", prop1)
				}

				if s, ok := prop2.([]interface{}); !ok || len(s) != 0 {
					t.Fatalf("expected `prop2` property to be an empty slice but got %v instead", prop2)
				}
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var serviceProps map[string]interface{}
			if err := json.Unmarshal(test.input, &serviceProps); err != nil {
				t.Fatal(err)
			}
			props := map[string]interface{}{
				"service1": serviceProps,
			}

			mockSSM := mockSSMService{
				Validator: test.validator,
				Response:  test.output,
			}
			getSSMClient = func(region string) secret.SSMAPI {
				return mockSSM
			}
			defer func() {
				getSSMClient = secret.GetSSMSession
			}()

			injectedProps, err := injectSecretsV2(log, props)
			test.expected(t, props, injectedProps, err)
		})
	}
}

func nestedMapLookup(m map[string]interface{}, keys ...string) (ret interface{}, err error) {
	if len(keys) == 0 {
		return nil, errors.New("at least one key is required")
	}

	if ret, ok := m[keys[0]]; !ok {
		return nil, fmt.Errorf("key `%s` not found; remaining keys: %v", keys[0], keys)
	} else if len(keys) == 1 {
		return ret, nil
	} else if m, ok := ret.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("nested structure was not a map: %#v", ret)
	} else {
		return nestedMapLookup(m, keys[1:]...)
	}
}
