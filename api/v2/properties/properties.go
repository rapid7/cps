package properties

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/rapid7/cps/pkg/kv"

	mux "github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	gjson "github.com/tidwall/gjson"
)

func init() {
	// logging
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

type Error struct {
	Status string `json:"status"`
}

func GetProperties(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	scope := strings.Split(vars["scope"], "/")
	service := scope[0]
	fullPath := scope[1:len(scope)]

	jsoni := kv.GetProperty(service)

	if jsoni == nil {
		return
	}

	jb := jsoni.([]byte)

	b := new(bytes.Buffer)
	if err := json.Compact(b, jb); err != nil {
		log.Error(err)
	}

	j := []byte(b.Bytes())

	// If fullPath is greater than 0 we are returning
	// a subset of the json if available. The else clause
	// returns the entire set of properties if available.
	if len(fullPath) > 0 {
		// Handle keys with "." in them. They need to be
		// escaped due to how gjson's pathing works. An
		// unescaped dot tells gjson to go a level deeper
		// into the json object. We don't want that if the
		// key itself has dots.
		for i, p := range fullPath {
			if strings.Contains(p, ".") {
				fullPath[i] = strings.Replace(p, ".", "\\.", -1)
			}
		}

		f := strings.Join(fullPath, ".")
		p := gjson.GetBytes(j, "properties")
		selected := gjson.GetBytes([]byte(p.String()), f)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(strings.TrimSpace(selected.String())))
	} else {
		w.Header().Set("Content-Type", "application/json")
		p := gjson.GetBytes(j, "properties")
		w.Write([]byte(strings.TrimSpace(p.String())))
	}
}
