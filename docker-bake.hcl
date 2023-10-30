variable "BUILD_AWS_ACCOUNT" {
}

variable "ECR_PREFIX" {
}

variable "JENKINS_REGION" {
}

variable "JOB_URL" {
}

variable "GIT_URL" {
}

variable "MAINTAINER" {
    default = "Platform Delivery"
}

variable "CONTAINER_SERVICE" {
}

variable "APP_VERSION" {
}

target "_common" {
  args = {
    JOB_URL    = JOB_URL
    GIT_URL    = GIT_URL
    MAINTAINER = MAINTAINER
  }
}

// Bake builds this by default
group "default" {
    targets = [
        "cps"
    ]
}

// cps
target "cps" {
    inherits = ["_common"]
    context = "."
    dockerfile = "Dockerfile"
    platforms = [
        "linux/amd64",
        "linux/arm64"
    ]
    tags = [
        "${BUILD_AWS_ACCOUNT}.dkr.ecr.${JENKINS_REGION}.amazonaws.com/${CONTAINER_SERVICE}:latest",
        "${BUILD_AWS_ACCOUNT}.dkr.ecr.${JENKINS_REGION}.amazonaws.com/${CONTAINER_SERVICE}:${APP_VERSION}"
    ]
}
