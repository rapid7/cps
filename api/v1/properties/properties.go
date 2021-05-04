package properties

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/rapid7/cps/kv"
)

// Error holds the data to be made into a json error message.
type Error struct {
	Status string `json:"status"`
}

// GetProperties is a mux handler for the /v1/properties endpoint. It returns all
// properties for a given service.
func GetProperties(w http.ResponseWriter, r *http.Request, account string, region string, log *zap.Logger) {
	vars := mux.Vars(r)
	service := vars["service"]

	var path bytes.Buffer
	path.WriteString(account)
	path.WriteString("/")
	path.WriteString(region)
	path.WriteString("/")
	path.WriteString(service)

	serviceProperties := kv.GetProperty(path.String()).(map[string]interface{})

	w.Header().Set("Content-Type", "application/json")

	if len(serviceProperties) < 1 {
		log.Error("Failed to get properties for service",
			zap.String("service", service),
		)

		e, _ := json.Marshal(Error{
			Status: "Failed to get properties for service",
		})

		// TODO: determine if a non-200 response code is acceptable in error cases
		// See https://github.com/golang/go/blob/master/src/net/http/server.go#L1538-L1539 specifically:
		// > Writing before sending a header sends an implicitly empty 200 OK header.
		// Thus, in order to implement HEAD request handlers we explicitly write the 200 and only write
		// on a GET request
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}

		w.Write(e)
		return
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
		log.Error("Failed to marshal json for a service",
			zap.Error(err),
			zap.String("service", service),
		)

		e, _ := json.Marshal(Error{
			Status: "failed to marshal json",
		})

		// TODO: See TODO above about explicitly setting response codes even in error conditions
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}

		w.Write(e)
		return
	}

	if r.Method == http.MethodHead {
		return
	}

	w.Write(j)
}

// GetProperty is a mux handler for getting a single property.
func GetProperty(w http.ResponseWriter, r *http.Request, account, region string, log *zap.Logger) {
	vars := mux.Vars(r)
	service := vars["service"]
	property := vars["property"]

	var path bytes.Buffer
	path.WriteString(account)
	path.WriteString("/")
	path.WriteString(region)
	path.WriteString("/")
	path.WriteString(service)

	serviceProperties := kv.GetProperty(path.String()).(map[string]interface{})
	serviceProperty := serviceProperties[property]

	var output bytes.Buffer
	var line string
	switch t := serviceProperty.(type) {
	case string:
		line = serviceProperty.(string)
	case int:
		line = strconv.Itoa(serviceProperty.(int))
	case bool:
		line = strconv.FormatBool(serviceProperty.(bool))
	case float64:
		line = strconv.FormatFloat(serviceProperty.(float64), 'f', -1, 64)
	case nil:
		line = "{}"
	default:
		log.Error("Unsupported type!",
			zap.String("key", property),
			zap.Any("value", serviceProperty),
			zap.Any("type", t),
		)

		line = "{}"
	}

	output.WriteString(line)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}

	w.Write(output.Bytes())
}
