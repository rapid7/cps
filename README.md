# CPS - Centralized Property Service

CPS is a centralized dynamic property service. It serves up the precomputed properties for a service as well as dynamic consul properties in the form of `conqueso.service.ips=`.

## configuration

In order to run this project you need to set the following values in `cps.json` at the root of the project:

```json
{
  "account": "000000000000",
  "region": "us-east-1",
  "s3": {
    "bucket": "mys3propertiesbucket",
    "region": "us-east-1"
  },
  "consul": {
    "host": "localhost:8500"
  }
}
```

## running locally

- `mkdir -p ~/go/src`
- add `export GOPATH=~/go` to your .bashrc or .zshrc
- git clone to `~/go/src/cps`
- `brew install go dep consul`
- `dep ensure`
- `make`
- `consul agent -dev -advertise 127.0.0.1` in another tab.
- export your awsaml creds
- `./cps`


## running on an ec2 instance

- `make build-linux`
- scp binary to the ec2 instance and run
