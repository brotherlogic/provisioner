protoc --proto_path ../../../ -I=./proto --go_out=plugins=grpc:./proto proto/provisioner.proto
mv proto/github.com/brotherlogic/provisioner/proto/* ./proto
