.PHONY: all test clean proto
proto:
	protoc --go_out=go proto/bb84.proto
