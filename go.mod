module github.com/rapid7/cps

go 1.14

require (
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/aws/aws-sdk-go v1.38.28
	github.com/buger/jsonparser v0.0.0-20180318095312-2cac668e8456
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/go-test/deep v1.0.7
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/consul/api v1.10.0
	github.com/hashicorp/consul/sdk v0.8.0 // indirect
	github.com/hashicorp/go-hclog v0.14.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/memberlist v0.3.1 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/miekg/dns v1.1.41 // indirect
	github.com/mitchellh/go-testing-interface v1.14.0 // indirect
	github.com/mitchellh/mapstructure v1.4.1
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	github.com/tidwall/gjson v1.1.3
	github.com/tidwall/match v1.0.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/sys v0.0.0-20220224120231-95c6836cb0e7 // indirect
	golang.org/x/tools v0.0.0-20210106214847-113979e3529a // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
)

// Viper includes github.com/bketelsen/crypt for remote k/v support (see
// https://github.com/spf13/viper/blob/v1.7.1/README.md#remote-keyvalue-store-support)
// This pulls in consul/api v1.1.0 which conflicts with our usage of consul v1.2.2.
// Because we don't use Viper's remote k/v, we can exclude the package entirely.
exclude github.com/hashicorp/consul/api v1.1.0
