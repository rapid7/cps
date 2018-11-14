package health

import (
	"encoding/json"
	"net/http"

	"cps/watchers/v2/s3"

	log "github.com/sirupsen/logrus"
)

type Response struct {
	Status string `json:"status"`
	S3     bool   `json:"s3"`
}

func GetHealthz(w http.ResponseWriter, r *http.Request) {
	var status string
	status = "down"
	if s3.Up {
		status = "up"
	}

	data, err := json.Marshal(Response{
		Status: status,
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
