package main

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"

	cq "github.com/rapid7/cps/api/v1/conqueso"
	health "github.com/rapid7/cps/api/v1/health"
	props "github.com/rapid7/cps/api/v1/properties"
	v2health "github.com/rapid7/cps/api/v2/health"
	v2props "github.com/rapid7/cps/api/v2/properties"
	kv "github.com/rapid7/cps/pkg/kv"
	consul "github.com/rapid7/cps/watchers/v1/consul"
	file "github.com/rapid7/cps/watchers/v1/file"
	s3 "github.com/rapid7/cps/watchers/v1/s3"
	v2file "github.com/rapid7/cps/watchers/v2/file"
	v2s3 "github.com/rapid7/cps/watchers/v2/s3"

	log "github.com/sirupsen/logrus"
)

func init() {
	// logging
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
}

func main() {

	log.Print("cps started")

	viper.SetConfigName("cps")
	viper.AddConfigPath("/etc/cps/")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Fatal error reading in config file: %s", err)
	}

	viper.SetDefault("file.enabled", false)
	fileEnabled := viper.GetBool("file.enabled")
	directory := viper.GetString("file.directory")

	account := viper.GetString("account")
	if account == "" {
		log.Fatalf("Config `account` is required!")
	}
	region := viper.GetString("region")
	if region == "" {
		log.Fatalf("Config `region` is required!")
	}
	bucket := viper.GetString("s3.bucket")
	if bucket == "" && !fileEnabled {
		log.Fatalf("Config `s3.bucket` is required!")
	}

	viper.SetDefault("s3.region", "us-east-1")
	bucketRegion := viper.GetString("s3.region")

	viper.SetDefault("consul.host", "localhost:8500")
	consulHost := viper.GetString("consul.host")

	viper.SetDefault("s3.enabled", true)
	s3Enabled := viper.GetBool("s3.enabled")

	viper.SetDefault("consul.enabled", true)
	consulEnabled := viper.GetBool("consul.enabled")

	viper.SetDefault("api.version", 1)
	apiVersion := viper.GetInt("api.version")

	viper.SetDefault("port", "9100")
	port := viper.GetString("port")

	router := mux.NewRouter()

	if apiVersion == 2 {
		router.HandleFunc("/v2/properties/{scope:.*}", func(w http.ResponseWriter, r *http.Request) {
			v2props.GetProperties(w, r)
		}).Methods("GET")

		if fileEnabled {
			log.Print("File mode is enabled, disabling s3 and consul watchers")
			s3Enabled = false
			consulEnabled = false
			go v2file.Poll(directory, account, region)
		}

		if s3Enabled {
			go v2s3.Poll(bucket, bucketRegion)
		}

		router.HandleFunc("/v2/healthz", v2health.GetHealthz).Methods("GET")

	} else {
		if fileEnabled {
			log.Print("File mode is enabled, disabling s3 and consul watchers")
			s3Enabled = false
			consulEnabled = false
			go file.Poll(directory, account, region)
		}

		if s3Enabled {
			go s3.Poll(bucket, bucketRegion)
		}

		if consulEnabled {
			go consul.Poll(consulHost)
		} else {
			kv.WriteProperty("consul", make(map[string][]string))
		}

		router.HandleFunc("/v1/properties/{service}", func(w http.ResponseWriter, r *http.Request) {
			props.GetProperties(w, r, account, region)
		}).Methods("GET")

		router.HandleFunc("/v1/conqueso/{service}", func(w http.ResponseWriter, r *http.Request) {
			cq.GetConquesoProperties(w, r, account, region)
		}).Methods("GET")

		router.HandleFunc("/v1/properties/{service}/{property}", func(w http.ResponseWriter, r *http.Request) {
			props.GetProperty(w, r, account, region)
		}).Methods("GET")

		router.HandleFunc("/v1/conqueso/{service}", cq.PostConqueso).Methods("POST")

		// Health returns detailed information about CPS health.
		router.HandleFunc("/v1/health", health.GetHealth).Methods("GET")
		// Healthz returns only basic health.
		router.HandleFunc("/v1/healthz", health.GetHealthz).Methods("GET")

	}

	// Serve it.
	log.Print(http.ListenAndServe(":"+port, router))

}
