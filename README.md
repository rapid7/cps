# CPS - Centralized Property Service

[![Go Report](https://goreportcard.com/badge/github.com/rapid7/cps)](https://goreportcard.com/badge/github.com/rapid7/cps)

CPS is a centralized dynamic property service. It serves up the precomputed properties for a service as well as dynamic consul properties in the form of `conqueso.service.ips=`. It also supports AWS SSM Parameter Store SecureStrings.

See [cps-property-generator](https://github.com/rapid7/cps-property-generator) for generating json property files and uploading them to s3.

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
- `brew install go consul`
- `make`
- `consul agent -dev -advertise 127.0.0.1` in another tab.
- export your awsaml creds
- `./cps`


## running on an ec2 instance

- `make build-linux`
- scp binary to the ec2 instance and run

## running a dev instance with local files

Sometimes it is desirable to run locally using flat files. You can put your service json files into a directory and point to that directory. Note, when file mode is enabled s3 and consul watchers are both disabled. Here is a basic example of how to set up your config for local files:

```
{
  "account": "0000000000000",
  "region": "us-east-1",
  "file": {
    "enabled": true,
    "directory": "./local-files"
  }
}
```

The names of the files in the `./local-files` should be the name of the service.

## running in docker

There is a Dockerfile at the root of the project that is meant to be used in local file mode. You can modify `dockerfiles/cps.json` and add/remove services from the `dockerfiles/services` directory to change what properties are returned. Here are the steps to get started quickly:

1. Uncomment dockerfiles ADD in Dockerfile
2. `docker build -t cps .`
3. `docker run -p 9100:9100 -it cps`
4. `curl localhost:9100/v1/properties/your-service`
