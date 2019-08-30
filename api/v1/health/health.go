package health

import (
	"encoding/json"
	"net/http"

	"github.com/rapid7/cps/watchers/v1/consul"
	"github.com/rapid7/cps/watchers/v1/s3"

	log "github.com/sirupsen/logrus"
)

// Struct wrapper for all health data.
type Health struct {
	Status  int           `json:"status"`
	Plugins HealthPlugins `json:"plugins"`
}

// Subset of the Health struct, it contains
// health information for various components.
type HealthPlugins struct {
	Consul bool `json:"consul"`
	S3     bool `json:"s3"`
}

// Handler for the health endpoint. Checks health for
// various components and returns the results as json.
func GetHealth(w http.ResponseWriter, r *http.Request) {
	var status int
	if s3.Health == true && consul.Health == true {
		status = 200
	} else {
		status = 503
	}

	data, err := json.Marshal(Health{
		Status: status,
		Plugins: HealthPlugins{
			Consul: consul.Health,
			S3:     s3.Health,
		},
	})
	if err != nil {
		log.Error(err)
		return
	}

	if status == 503 {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
