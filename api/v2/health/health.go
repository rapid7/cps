package health

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/rapid7/cps/watchers/v2/s3"
)

// Response holds the json response for /v2/healthz.
type Response struct {
	Status string `json:"status"`
	S3     bool   `json:"s3"`
}

// GetHealthz returns the basic health status as json.
func GetHealthz(w http.ResponseWriter, r *http.Request, log *zap.Logger) {
	status := "down"
	if s3.Up {
		status = "up"
	}

	w.Header().Set("Content-Type", "application/json")

	data, err := json.Marshal(Response{
		Status: status,
		S3:     s3.Up,
	})
	if err != nil {
		log.Error("Failed to unmarshal json",
			zap.Error(err),
		)
		w.WriteHeader(http.StatusInternalServerError)
		if r.Method == http.MethodHead {
			return
		}

		w.Write([]byte(`{}`)) //nolint: errcheck
		return
	}

	if status == "down" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if r.Method == http.MethodHead {
		return
	}
	w.Write(data) //nolint: errcheck
}
