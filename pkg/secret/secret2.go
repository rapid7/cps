package secret

import (
	"encoding/base64"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"

	log "github.com/sirupsen/logrus"
	openssl "github.com/spacemonkeygo/openssl"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func Decrypt2(s []byte) (string, error) {
	region := "us-east-1"

	var j map[string]interface{}
	err := json.Unmarshal(s, &j)
	if err != nil {
		log.Errorf("Failed to unmarshal object: %v", err)
	}

	var ct []byte
	var dk []byte
	if _, ok := j["$tokend"]; ok {
		data := j["$tokend"].(map[string]interface{})
		log.Print(data["datakey"].(string))
		dk, err = base64.StdEncoding.DecodeString(data["datakey"].(string))
		if err != nil {
			log.Errorf("error: %v", err)
		}
		ct, err = base64.StdEncoding.DecodeString(data["ciphertext"].(string))
		if err != nil {
			log.Errorf("error: %v", err)
		}
	} else {
		log.Errorf("Object is not a tokend stanza: %v", j)
		// TODO: populate err
		return "", err
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}))

	svc := kms.New(sess)

	params := &kms.DecryptInput{
		CiphertextBlob: dk,
	}

	dek, err := svc.Decrypt(params)
	if err != nil {
		log.Errorf("Error decrypting datakey: %v", err)
		return "", err
	}

	iv := make([]byte, 16)
	crypt, err := NewCrypter(dek.Plaintext, iv)

	pt, err := crypt.decryptCBC(ct)

	return string(pt), nil
}

type Crypter struct {
	key    []byte
	iv     []byte
	cipher *openssl.Cipher
}

func NewCrypter(key []byte, iv []byte) (*Crypter, error) {
	cipher, err := openssl.GetCipherByName("aes-256-cbc")
	if err != nil {
		return nil, err
	}

	return &Crypter{key, iv, cipher}, nil
}

func (c *Crypter) decryptCBC(ciphertext []byte) ([]byte, error) {
	ctx, err := openssl.NewDecryptionCipherCtx(c.cipher, nil, c.key, c.iv)
	if err != nil {
		log.Errorf("Error creating encryption ctx: %v", err)
		return nil, err
	}

	cipherbytes, err := ctx.DecryptUpdate(ciphertext)
	if err != nil {
		log.Errorf("Error updating decryption: %v", err)
		return nil, err
	}

	finalbytes, err := ctx.DecryptFinal()
	if err != nil {
		log.Errorf("Error finalizing decryption: %v", err)
		return nil, err
	}

	cipherbytes = append(cipherbytes, finalbytes...)

	log.Printf("CBYTES: %s", cipherbytes)

	return cipherbytes, nil
}
