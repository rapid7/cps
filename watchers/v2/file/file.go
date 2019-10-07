package file

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rapid7/cps/kv"
)

var (
	// Config is a global reference to the config struct. The struct just
	// needs to be exported (TODO).
	Config config
)

type config struct {
	directory string
	account   string
	region    string
}

// Poll constructs a poller for files in the directory supplied.
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

// Sync traverses all files in Config.directory and writes them
// to the kv store.
func Sync(t time.Time, log *zap.Logger) {
	absPath, _ := filepath.Abs(Config.directory)

	files, err := ioutil.ReadDir(absPath)
	if err != nil {
		log.Error("Error reading directory",
			zap.Error(err),
			zap.String("static_file_dir", absPath),
		)

		return
	}

	for _, f := range files {
		fn := f.Name()
		if strings.Contains(fn, ".json") {
			fullPath := absPath + "/" + fn
			shortPath := fn[0 : len(fn)-5]

			jsonBytes, err := ioutil.ReadFile(fullPath)
			if err != nil {
				log.Error("Failed to read json file",
					zap.Error(err),
					zap.String("filename", fullPath),
				)

				return
			}

			kv.WriteProperty(shortPath, jsonBytes)
		} else {
			log.Error("File does not have the json extension",
				zap.String("filename", fn),
			)

			return
		}
	}

}
