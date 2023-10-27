pipeline {
    agent {
        kubernetes(
            k8sAgent(
                dindCPU: '8',
                dindMEM: '8Gi',
                arch: 'amd64', // Issues with building amd64 image on arm64 hardware
                idleMinutes: params.POD_IDLE_MINUTES // Pod will stay idle post build for this amount of minutes
            )
        )
    }

    options {
        buildDiscarder logRotator(artifactDaysToKeepStr: '', artifactNumToKeepStr: '', daysToKeepStr: '', numToKeepStr: '20')
        datadog(collectLogs: true,
            tags:
                ["pipeline:cps",
                "language:go",
                "buildtool:docker"])
        ansiColor('xterm')
        timestamps()
        timeout(360) // Mins
    }

    parameters {
        string(name: 'POD_IDLE_MINUTES', defaultValue: '0', description: 'Number of minutes pod will stay idle post build')
        booleanParam(name: 'PUSH_TO_ECR', defaultValue: false, description: 'Push images to ECR repos')
    }

    environment {
        ECR_REGISTRY_BUILD        = "${env.BUILD_AWS_ACCOUNT}.dkr.ecr.${env.JENKINS_REGION}.amazonaws.com"
        ECR_LOGIN_CMD             = "aws ecr get-login-password --region ${env.JENKINS_REGION} | docker login --username AWS --password-stdin"
        ARTIFACTORY_DOCKER_REPO = 'docker-local.artifacts.corp.rapid7.com'
        BAKE_PUSH               = "${params.PUSH_TO_ECR == false ? "" : "--push"}"
        BAKE_CMD                = "BUILD_AWS_ACCOUNT=${env.BUILD_AWS_ACCOUNT} \
                                    JENKINS_REGION=${env.JENKINS_REGION} \
                                    GIT_URL=${env.GIT_URL} \
                                    JOB_URL=${env.JOB_URL} \
                                    docker buildx bake --file docker-bake.hcl ${env.BAKE_PUSH}"
    }

    stages {
        stage('Set build parameters') {
            steps {
                script {
                    env.CONTAINER_SERVICE = env.GIT_URL.replaceFirst(/^.*\/([^\/]+?).git$/, '$1')
                    def build_timestamp = new Date().format("yyyyMMddHHmm", TimeZone.getTimeZone('UTC'))
                    env.APP_VERSION = getSafeName("${BRANCH_NAME}-${build_timestamp}-${BUILD_NUMBER}")
                    println "Container Service: ${env.CONTAINER_SERVICE}"
                    println "App Version: ${env.APP_VERSION}"
                }
            }
        }

        stage('Login to Repos') {
            steps {
                sh label: "Log in to build account ECR", script: "${ECR_LOGIN_CMD} ${ECR_REGISTRY_BUILD}"
                withCredentials([usernamePassword(credentialsId: 'artifactory-deployment-credentials', usernameVariable: 'ARTIFACTORY_USERNAME', passwordVariable: 'ARTIFACTORY_PWD')]) {
                    sh label: "Log in to artifactory repository", script: 'docker login -u "${ARTIFACTORY_USERNAME}" -p "${ARTIFACTORY_PWD}" "${ARTIFACTORY_DOCKER_REPO}"'
                }
            }
        }


        stage('Docker buildx create') {
            steps {
                sh label: 'Docker buildx create',
                script: """
                    docker run -d --rm --privileged tonistiigi/binfmt --install all
                    docker context create tls-env
                    docker buildx create --config=/etc/buildkit/buildkitd.toml --name ${env.BUILD_TAG} --use tls-env
                    docker buildx ls
                """
            }
        }

        stage('Docker buildx bake') {
            steps {
                sh label: 'Docker buildx bake',
                script: """
                    if [ ${env.BAKE_PUSH} != null ]
                    then
                        ${env.BAKE_CMD} --push
                    else
                        ${env.BAKE_CMD}
                    fi
                """
            }
        }

        stage('Sign Images') {
            when {
                allOf {
                    expression { return params.PUSH_TO_ECR }
                    //branch 'master'
                    not { changeRequest() }
                }
            }
            steps {
                multiArchCosign(
                    sourceImageName: env.CONTAINER_SERVICE,
                    sourceImageTag: env.APP_VERSION
                )
            }
        }

        stage('Publish to ecr targets') {

            stages {

                stage('release_target') {
                    when {
                        allOf {
                            expression { return params.PUSH_TO_ECR }
                            //branch 'master'
                            not { changeRequest() }
                        }
                    }
                    steps {
                        script {
                            def release_envs = [:]
                            def yamlFile = readYaml (file: "ecr_targets.yaml")
                            yamlFile.get('release_target').each { target ->
                                def cloud_name = target.getKey()
                                def account_id = target.getValue().get('account_id')
                                def iam_role = target.getValue().get('iam_role')
                                def regions = target.getValue().get('regions')

                                release_envs["$cloud_name"] = {
                                    stage("Push RELEASE image(s) to $cloud_name") {
                                        catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE') {
                                            ecrCopy(
                                                targetAccount: account_id,
                                                targetRegions: regions,
                                                sourceImageName: env.CONTAINER_SERVICE,
                                                sourceImageTag: env.APP_VERSION,
                                                autoTagLatest: autotag_latest,
                                                stsAssumeWebIdentity: true,
                                                iamRoleArn: iam_role,
                                                assumeScope: 'local',
                                                credFile: true,
                                                setAwsCredFileEnv: true,
                                            )
                                        }
                                    }
                                }
                            }
                            parallel release_envs
                        }
                    }
                }

                stage('fedramp') {
                    when {
                        allOf {
                            expression { return params.PUSH_TO_ECR }
                            //branch 'master'
                            not { changeRequest() }
                        }
                    }
                    steps {
                        script {
                            def fedramp_envs = [:]
                            def yamlFile = readYaml (file: "ecr_targets.yaml")
                            yamlFile.get('fedramp').each { target ->
                                def cloud_name = target.getKey()
                                def account_id = target.getValue().get('account_id')
                                def iam_role = target.getValue().get('iam_role')
                                def regions = target.getValue().get('regions')
                                fedramp_envs["$cloud_name"] = {
                                    stage("Push FEDRAMP image(s) to $cloud_name") {
                                        catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE') {
                                            ecrCopy(
                                                targetAccount: account_id,
                                                targetRegions: regions,
                                                sourceImageName: env.CONTAINER_SERVICE,
                                                sourceImageTag: "${env.APP_VERSION}-fips",
                                                autoTagLatest: autotag_latest,
                                                stsAssumeWebIdentity: true,
                                                iamRoleArn: iam_role,
                                                assumeScope: 'local',
                                                credFile: true,
                                                setAwsCredFileEnv: true,
                                            )
                                        }
                                    }
                                }
                            }
                            parallel fedramp_envs
                        }
                    }
                }
            } // End Publish to ecr targets stages
        } // End of Publish to ecr targets stage

    } // End stages
} // End pipeline
