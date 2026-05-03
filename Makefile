.PHONY: gen
gen:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.1
	cd shared/proto && protoc --go_out=../ --go_opt=paths=source_relative game.proto
