test:
	go test ./parse -v

testcover:
	go test ./parse -coverprofile=coverage.out
	go tool cover -html=coverage.out
