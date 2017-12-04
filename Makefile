setup:
	go get -u github.com/golang/lint/golint
	mkdir -p ${GOSRC}
	ln -s ${WORKSPACE}/ ${GOSRC}/${PROJECT}

vet:
	go tool vet main.go parse/*.go graph/*.go

lint:
	${GOPATH}/bin/golint ./

test:
	go test ./parse -v

testcover:
	go test ./parse -coverprofile=coverage.out
	go tool cover -html=coverage.out
