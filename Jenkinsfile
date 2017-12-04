#!groovy

pipeline {
    agent any

    environment {
        GOROOT = tool name: 'Go 1.9', type: 'go'
        GOPATH = "${JENKINS_HOME}/workspace/${JOB_NAME}/${BUILD_ID}"
        GOSRC = "${GOPATH}/src/github.com/psql-schema-dump-sanitiser.git"
    }

    stages {
        stage('Pre-Test') {
            steps {
                withEnv(["PATH+GO=${GOROOT}/bin"]) {
                    echo 'Setup Environment'

                    sh 'go version'
                    sh 'make setup'
                }
            }
        }

        stage('Vet') {
            steps {
                withEnv(["PATH+GO=${GOROOT}/bin"]) {
                    echo 'Vetting Code'

                    sh 'cd $GOSRC && make vet'
                }
            }
        }

        stage('Lint') {
            steps {
                withEnv(["PATH+GO=${GOROOT}/bin"]) {
                    echo 'Linting Code'

                    sh 'cd $GOSRC && make lint'
                }
            }
        }

        stage('Test') {
            steps {
                withEnv(["PATH+GO=${GOROOT}/bin"]) {
                    echo 'Tests'

                    sh 'cd $GOSRC && make test'
                }
            }
        }

        stage('Cleanup') {
            steps {
                cleanWs()
            }
        }
    }
}
