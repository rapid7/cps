package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func Decrypt(s []byte) (string, error) {
	// TODO: Get region from config
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

	pt, err := decryptCBC(dek.Plaintext, ct)
	if err != nil {
		log.Errorf("Error decrypting ciphertext: %v", err)
	}

	log.Printf("The secret: %s", pt)

	return string(pt[:]), nil
}

func decryptCBC(key, ciphertext []byte) (plaintext []byte, err error) {

	var block cipher.Block
	if block, err = aes.NewCipher(key); err != nil {
		return
	}

	if len(ciphertext) < aes.BlockSize {
		log.Error("ciphertext too short")
		return
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		panic("ciphertext is not a multiple of the block size")
	}

	iv := make([]byte, 16)
	deciphered := make([]byte, 16)
	cbc := cipher.NewCBCDecrypter(block, iv)
	cbc.CryptBlocks(deciphered, ciphertext)

	return deciphered, nil
}
