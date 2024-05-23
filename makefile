build:
	@go build -o bin/rvc-user cmd/user/main.go
	@go build -o bin/rvc-session cmd/session/main.go

run-user:
	@./bin/rvc-user

run-session:
	@./bin/rvc-session

clean:
	@rm -rf bin