package health

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/rapid7/cps/watchers/v1/consul"
	"github.com/rapid7/cps/watchers/v1/s3"
)

// Health is a wrapper for all health data.
type Health struct {
	Status  int           `json:"status"`
	Plugins HealthPlugins `json:"plugins"`
}

// HealthPlugins is a child of the Health struct. It currently contains health
// status for consul and S3.
type HealthPlugins struct {
	Consul bool `json:"consul"`
	S3     bool `json:"s3"`
}

// GetHealth is a mux handler for the health endpoint. It checks health for
// various components and returns the results as json.
func GetHealth(w http.ResponseWriter, r *http.Request, log *zap.Logger, consulEnabled bool) {
	var status int
	if (s3.Health == true && !consulEnabled) || (s3.Health == true && consul.Health == true && consulEnabled) {
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
		log.Error("Failed to marshal json!",
			zap.Error(err),
		)

		return
	}

	if status == 503 {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		return
	}

	w.Write(data)
}
