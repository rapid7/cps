package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	kv "cps/pkg/kv"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.Print("starting v2 file watcher...")
}

var (
	Config config
)

type config struct {
	directory string
	account   string
	region    string
}

func Poll(directory, account, region string) {
	Config = config{
		directory: directory,
		account:   account,
		region:    region,
	}

	Sync(time.Now())

	ticker := time.NewTicker(60 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				Sync(time.Now())
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func Sync(t time.Time) {
	absPath, _ := filepath.Abs(Config.directory)

	files, err := ioutil.ReadDir(absPath)
	if err != nil {
		log.Errorf("Error reading directory: %v", err)
		return
	}

	for _, f := range files {
		fn := f.Name()
		if strings.Contains(fn, ".json") {
			fullPath := absPath + "/" + fn
			shortPath := fn[0 : len(fn)-5]

			jsonBytes, err := ioutil.ReadFile(fullPath)
			if err != nil {
				log.Error(err)
				return
			}

			kv.WriteProperty(shortPath, jsonBytes)
		} else {
			log.Errorf("File does not have the json extension: %v", fn)
			return
		}
	}

}
