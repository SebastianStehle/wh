protoc --go_out=../server/domain/areas/tunnel/ --go-grpc_out=../server/domain/areas/tunnel/ service.proto

protoc --go_out=../client/ --go-grpc_out=../client/ service.proto