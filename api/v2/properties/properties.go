package properties

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	mux "github.com/gorilla/mux"
	gjson "github.com/tidwall/gjson"

	"github.com/rapid7/cps/kv"
)

// Error is unused currently but it intended to supply a detailed
// error message when a GET fails (TODO).
type Error struct {
	Status string `json:"status"`
}

// GetProperties is a handler for the /v2/properties/{service}/* endpoint. It
// can return all properties for a service or a subset of properties if
// additional paths are given after {service}.
func GetProperties(w http.ResponseWriter, r *http.Request, log *zap.Logger) {
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
		log.Error("Failed to compact json",
			zap.Error(err),
		)
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
