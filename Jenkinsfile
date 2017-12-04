#!groovy

pipeline {
    agent any

    environment {
        GOROOT = tool name: 'Go 1.9', type: 'go'
        PROJECT = "psql-schema-dump-sanitiser"
        GOPATH = "${JENKINS_HOME}/workspace/${JOB_NAME}/${BUILD_ID}"
        GOSRC = "${GOPATH}/src/github.com/jchiam"
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

                    sh 'cd $GOSRC/$PROJECT && make vet'
                }
            }
        }

        stage('Lint') {
            steps {
                withEnv(["PATH+GO=${GOROOT}/bin"]) {
                    echo 'Linting Code'

                    sh 'cd $GOSRC/$PROJECT && make lint'
                }
            }
        }

        stage('Test') {
            steps {
                withEnv(["PATH+GO=${GOROOT}/bin"]) {
                    echo 'Tests'

                    sh 'cd $GOSRC/$PROJECT && make test'
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
