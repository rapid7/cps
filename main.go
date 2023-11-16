package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/rapid7/cps/api"
	cq "github.com/rapid7/cps/api/v1/conqueso"
	"github.com/rapid7/cps/api/v1/health"
	props "github.com/rapid7/cps/api/v1/properties"
	v2health "github.com/rapid7/cps/api/v2/health"
	v2props "github.com/rapid7/cps/api/v2/properties"
	"github.com/rapid7/cps/kv"
	"github.com/rapid7/cps/logger"
	"github.com/rapid7/cps/watchers/v1/consul"
	"github.com/rapid7/cps/watchers/v1/file"
	"github.com/rapid7/cps/watchers/v1/s3"
	v2file "github.com/rapid7/cps/watchers/v2/file"
	v2s3 "github.com/rapid7/cps/watchers/v2/s3"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "", "(Optional) Config file")
	flag.StringVar(&configFile, "c", "", "(Optional) Config file")
	flag.Parse()
	viper.SetConfigName("cps")
	viper.AddConfigPath("/etc/cps/")
	viper.AddConfigPath(".")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	}
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Sprintf("Cannot read config file: %s. Will use ENV variables if present", err)
	}
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetEnvPrefix("cps")
	viper.AutomaticEnv()
	fmt.Printf("s3.bucket=%v\n", viper.Get("s3.bucket"))
	fmt.Printf("consul.enabled=%v\n", viper.GetBool("consul.enabled"))
	fmt.Printf("api.version=%v\n", viper.GetInt("api.version"))

	logOpts := make([]logger.ConfigOption, 0)
	logLevel := viper.GetString("log.level")
	if logLevel != "" {
		// translate string level to type
		var l zapcore.Level
		if err := l.UnmarshalText([]byte(logLevel)); err != nil {
			panic(fmt.Sprintf("Fatal error attempting to set log level: %s", err))
		}
		logOpts = []logger.ConfigOption{logger.ConfigWithLevel(l)}
	}

	devMode := viper.GetBool("dev")
	if devMode {
		logOpts = []logger.ConfigOption{
			logger.ConfigWithDevelopmentMode(),
		}
	}

	log := logger.BuildLogger(logOpts...)
	defer log.Sync() //nolint: errcheck

	viper.SetDefault("file.enabled", false)
	fileEnabled := viper.GetBool("file.enabled")
	directory := viper.GetString("file.directory")

	account := viper.GetString("account")
	if account == "" {
		log.Fatal("Config `account` is required!")
	}
	region := viper.GetString("region")
	if region == "" {
		log.Fatal("Config `region` is required!")
	}
	bucket := viper.GetString("s3.bucket")
	if bucket == "" && !fileEnabled {
		log.Fatal("Config `s3.bucket` is required!")
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

	log.Info("CPS started")

	router := mux.NewRouter()

	if apiVersion == 2 {
		router.HandleFunc("/v2/properties/{scope:.*}", func(w http.ResponseWriter, r *http.Request) {
			v2props.GetProperties(w, r, log)
		}).Methods(http.MethodGet, http.MethodHead)

		if fileEnabled {
			log.Info("File mode is enabled, disabling s3 and consul watchers")

			s3Enabled = false

			go v2file.Poll(directory, account, region, log)
		}

		if s3Enabled {
			viper.SetDefault("secret.version", int(v2s3.V1))
			secretVersion := viper.GetInt("secret.version")
			sv := v2s3.SecretHandlerVersion(secretVersion)
			go v2s3.Poll(bucket, bucketRegion, sv, log)
		}

		router.HandleFunc("/v2/healthz", func(w http.ResponseWriter, r *http.Request) {
			v2health.GetHealthz(w, r, log)
		}).Methods(http.MethodGet, http.MethodHead)

	} else {
		if fileEnabled {
			log.Info("File mode is enabled, disabling s3 and consul watchers")

			s3Enabled = false
			consulEnabled = false

			go file.Poll(directory, account, region, log)
		}

		if s3Enabled {
			go s3.Poll(bucket, bucketRegion, log)
		}

		if consulEnabled {
			go consul.Poll(consulHost, log)
		} else {
			kv.WriteProperty("consul", make(map[string][]string))
		}

		router.HandleFunc("/v1/properties/{service}", func(w http.ResponseWriter, r *http.Request) {
			props.GetProperties(w, r, account, region, log)
		}).Methods(http.MethodGet, http.MethodHead)

		router.HandleFunc("/v1/conqueso/{service}", func(w http.ResponseWriter, r *http.Request) {
			cq.GetConquesoProperties(w, r, account, region, log)
		}).Methods(http.MethodGet, http.MethodHead)

		router.HandleFunc("/v1/properties/{service}/{property}", func(w http.ResponseWriter, r *http.Request) {
			props.GetProperty(w, r, account, region, log)
		}).Methods(http.MethodGet, http.MethodHead)

		router.HandleFunc("/v1/conqueso/{service}", cq.PostConqueso).Methods(http.MethodPost, http.MethodHead)

		// Health returns detailed information about CPS health.
		router.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
			health.GetHealth(w, r, log, consulEnabled)
		}).Methods(http.MethodGet, http.MethodHead)

		// Healthz returns only basic health.
		router.HandleFunc("/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
			health.GetHealthz(w, r, log, consulEnabled)
		}).Methods(http.MethodGet, http.MethodHead)
	}

	if devMode {
		fmt.Println("\nRoutes:")
		if err := router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			if route.GetHandler() == nil {
				return nil
			}

			methods, err := route.GetMethods()
			if err != nil {
				return err
			}

			template, err := route.GetPathTemplate()
			if err != nil {
				return err
			}

			fmt.Println(methods, template)

			return nil
		}); err != nil {
			log.Fatal("error walking routes",
				zap.Error(err),
			)
		}
		fmt.Println("")
	}

	// Add request logging middleware and recovery handler
	h := api.RequestLoggingMiddleware(log)(
		handlers.RecoveryHandler(
			handlers.PrintRecoveryStack(true),
		)(router),
	)

	// Serve it.
	log.Fatal("Failed to attach to port",
		zap.Error(http.ListenAndServe(":"+port, h)),
	)

}
