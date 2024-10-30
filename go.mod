module github.com/rapid7/cps

go 1.21.3

require (
	github.com/aws/aws-sdk-go v1.45.27
	github.com/buger/jsonparser v1.1.1
	github.com/go-test/deep v1.0.7
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/consul/api v1.1.0
	github.com/hashicorp/consul/sdk v0.14.1
	github.com/mitchellh/mapstructure v1.5.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.8.3
	github.com/tidwall/gjson v1.17.0
	go.uber.org/zap v1.16.0
)

require (
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fatih/color v1.14.1 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.2.1 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/miekg/dns v1.1.50 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/pelletier/go-toml v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.5.0 // indirect
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Viper includes github.com/bketelsen/crypt for remote k/v support (see
// https://github.com/spf13/viper/blob/v1.7.1/README.md#remote-keyvalue-store-support)
// This pulls in consul/api v1.1.0 which conflicts with our usage of consul v1.2.2.
// Because we don't use Viper's remote k/v, we can exclude the package entirely.
exclude github.com/hashicorp/consul/api v1.25.1

replace github.com/armon/go-metrics => github.com/hashicorp/go-metrics v0.5.1
