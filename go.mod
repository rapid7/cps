module github.com/rapid7/cps

go 1.14

require (
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/aws/aws-sdk-go v1.42.34
	github.com/buger/jsonparser v0.0.0-20180318095312-2cac668e8456
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-test/deep v1.0.7
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/consul/api v1.18.0
	github.com/hashicorp/go-hclog v1.2.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/mitchellh/mapstructure v1.4.3
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pelletier/go-toml v1.7.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.8.0
	github.com/tidwall/gjson v1.1.3
	github.com/tidwall/match v1.0.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/net v0.4.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
)

// Viper includes github.com/bketelsen/crypt for remote k/v support (see
// https://github.com/spf13/viper/blob/v1.7.1/README.md#remote-keyvalue-store-support)
// This pulls in consul/api v1.1.0 which conflicts with our usage of consul v1.2.2.
// Because we don't use Viper's remote k/v, we can exclude the package entirely.
exclude github.com/hashicorp/consul/api v1.1.0
