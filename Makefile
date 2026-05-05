.PHONY: proto
proto:
	cd shared/proto && protoc --go_out=../ --go_opt=paths=source_relative game.proto
