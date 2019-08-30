package health

import (
	"encoding/json"
	"net/http"

	"github.com/rapid7/cps/watchers/v1/consul"
	"github.com/rapid7/cps/watchers/v1/s3"

	log "github.com/sirupsen/logrus"
)

// Holds the json response for the /v1/healthz endpoint.
type Response struct {
	Status string `json:"status"`
	Consul bool   `json:"consul"`
	S3     bool   `json:"s3"`
}

// Handler for the /v1/healthz endpoint. It returns detailed
// health information about all dependent services.
func GetHealthz(w http.ResponseWriter, r *http.Request) {
	status := "down"
	if s3.Up == true && consul.Up == true {
		status = "up"
	}

	data, err := json.Marshal(Response{
		Status: status,
		Consul: consul.Up,
		S3:     s3.Up,
	})
	if err != nil {
		log.Error(err)
	}

	if status == "down" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
