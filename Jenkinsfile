#!groovy

pipeline {
    agent any
    
    environment {
        root = tool name: 'Go 1.9', type: 'go'
    }

    stages {
        stage('Test') {
            steps {
                withEnv(["GOROOT=${root}", "PATH+GO=${root}/bin"]) {
                    sh 'make test'
                }
            }
        }
    }
}
