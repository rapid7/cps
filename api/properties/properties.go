package properties

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"

	mux "github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	kv "cps/pkg/kv"
)

func init() {
	// logging
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

type Error struct {
	Status string `json:"status"`
}

func GetProperties(w http.ResponseWriter, r *http.Request, account string, region string) {
	vars := mux.Vars(r)
	service := vars["service"]

	var path bytes.Buffer
	path.WriteString(account)
	path.WriteString("/")
	path.WriteString(region)
	path.WriteString("/")
	path.WriteString(service)

	serviceProperties := kv.GetProperty(path.String()).(map[string]interface{})

	if len(serviceProperties) < 1 {
		log.Errorf("Failed to get properties for service: %v", service)
		w.Header().Set("Content-Type", "application/json")
		e, _ := json.Marshal(Error{
			Status: "Failed to get properties for service",
		})
		w.Write(e)
	}

	combinedProperties := make(map[string]interface{})
	for k, v := range serviceProperties {
		combinedProperties[k] = v
	}

	consulProperties := kv.GetProperty("consul").(map[string][]string)
	combinedProperties["consul"] = make(map[string][]string)
	for k, v := range consulProperties {
		combinedProperties["consul"].(map[string][]string)[k] = v
	}

	j, err := json.Marshal(combinedProperties)
	if err != nil {
		log.Errorf("Failed to marshal json for %v", service)
		w.Header().Set("Content-Type", "application/json")
		e, _ := json.Marshal(Error{
			Status: "failed to marshal json",
		})
		w.Write(e)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(j)
	}
}
