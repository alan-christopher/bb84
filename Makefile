.PHONY: all test clean proto

# TODO: use --go_opt=MODULE instead of hacking the relpaths like this:
#   https://developers.google.com/protocol-buffers/docs/reference/go-generated
proto:
	protoc --go_out=go proto/bb84.proto
