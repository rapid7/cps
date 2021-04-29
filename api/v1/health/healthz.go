package health

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/rapid7/cps/watchers/v1/consul"
	"github.com/rapid7/cps/watchers/v1/s3"
)

// Response Holds the json response for the /v1/healthz endpoint.
type Response struct {
	Status string `json:"status"`
	Consul bool   `json:"consul"`
	S3     bool   `json:"s3"`
}

// GetHealthz is a mux handler for the /v1/healthz endpoint. It returns detailed
// health information about all dependent services.
func GetHealthz(w http.ResponseWriter, r *http.Request, log *zap.Logger, consulEnabled bool) {
	status := "down"
	if (s3.Up == true && !consulEnabled) || (s3.Up == true && consul.Up == true && consulEnabled) {
		status = "up"
	}

	data, err := json.Marshal(Response{
		Status: status,
		Consul: consul.Up,
		S3:     s3.Up,
	})

	if err != nil {
		log.Error("Failed to marshal json",
			zap.Error(err),
		)
	}

	if status == "down" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
