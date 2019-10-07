package file

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	"go.uber.org/zap"

	"github.com/rapid7/cps/kv"
	"github.com/rapid7/cps/secret"
)

var (
	// Config is a global for the config struct. The config
	// struct below should just be exported (TODO).
	Config config
)

type config struct {
	directory string
	account   string
	region    string
}

// Poll polls every 60 seconds, causing the application
// to parse the files in the supplied directory.
func Poll(directory, account, region string, log *zap.Logger) {
	Config = config{
		directory: directory,
		account:   account,
		region:    region,
	}

	Sync(time.Now(), log)

	ticker := time.NewTicker(60 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				Sync(time.Now(), log)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

// Sync performs the actual work of traversing the supplied
// directory and adding properties to the kv store.
func Sync(t time.Time, log *zap.Logger) {
	absPath, _ := filepath.Abs(Config.directory)

	files, err := ioutil.ReadDir(absPath)
	if err != nil {
		log.Error("Error reading directory",
			zap.Error(err),
			zap.String("directory", absPath),
		)

		return
	}

	for _, f := range files {
		properties := make(map[string]interface{})
		fn := f.Name()

		// Removes .json extension.
		shortPath := fn[0 : len(fn)-5]
		fullPath := absPath + "/" + fn
		path := Config.account + "/" + Config.region + "/" + shortPath

		j, _ := ioutil.ReadFile(fullPath)
		jsonparser.ObjectEach(j, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			switch dataTypeString := dataType.String(); dataTypeString {
			case "string":
				properties[string(key)] = string(value)
			case "number":
				var v interface{}
				if strings.Contains(string(value), ".") {
					v, _ = strconv.ParseFloat(string(value), 64)
				} else {
					v, _ = strconv.Atoi(string(value))
				}
				properties[string(key)] = v
			case "boolean":
				v, _ := strconv.ParseBool(string(value))
				properties[string(key)] = v
			case "null":
				properties[string(key)] = ""
			case "object":
				s, err := secret.GetSSMSecret(string(key), value)
				if err != nil {
					log.Error("Failed to get SSM secret",
						zap.Error(err),
					)

					return err
				}
				properties[string(key)] = s
			default:
				log.Error("Unsupported type!",
					zap.String("service", shortPath),
					zap.String("key", string(key)),
					zap.String("value", string(value)),
					zap.String("type", dataTypeString),
				)
			}

			return nil
		}, "properties")

		kv.WriteProperty(path, properties)
	}
}
