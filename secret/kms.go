package secret

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
)

const (
	// KMSIdentifier is the magic string identifying a KMS secret stanza
	KMSIdentifier = "$kms"
)

var (
	// ErrKMSMissingRegion is a typed error if a KMS stanza is missing a region
	ErrKMSMissingRegion = errors.New("KMS credential is missing the region key")
)

// KMS is a plain-old-Go-object for carrying structured KMS stanzas in CPS props
type KMS struct {
	KMS struct {
		Region    string `mapstructure:"region"`
		Encrypted string `mapstructure:"encrypted"`
	} `mapstructure:"$kms"`
}

// KMSAPI is a local wrapper over aws-sdk-go's KMS API
type KMSAPI interface {
	kmsiface.KMSAPI
}

// DecryptKMSSecret decrypts a KMS encrypted secret
func DecryptKMSSecret(ctx context.Context, svc KMSAPI, ciphertext string) (string, error) {
	ciphertextBlob, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	resp, err := svc.DecryptWithContext(ctx, &kms.DecryptInput{
		CiphertextBlob: ciphertextBlob,
	})
	if err != nil {
		return "", err
	}

	return string(resp.Plaintext), nil
}

// GetKMSSession gets a regional KMS session
func GetKMSSession(region string) KMSAPI {
	var svc KMSAPI = kms.New(getSession(region))
	return svc
}
