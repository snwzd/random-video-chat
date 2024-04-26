build:
	@go build -o bin/rvc-user cmd/user_service/main.go
	@go build -o bin/rvc-userconn cmd/userconnection_service/main.go
	@go build -o bin/rvc-forwarder cmd/forwarder_service/main.go
	@go build -o bin/rvc-userevent cmd/userevent_service/main.go

run-user:
	@./bin/rvc-user

run-userconn:
	@./bin/rvc-userconn

run-forwarder:
	@./bin/rvc-forwarder

run-userevent:
	@./bin/rvc-userevent

clean:
	@rm -rf bin