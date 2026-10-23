Handin 2 is for help when coding (will be removed)

chit chat is the actual code




## Regenerate chitchat_grpc.pb.go and chitchat.pb.go when changing chitchat.proto

When changing the chitchat proto remember to regenerate the two files using:

protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/chitchat.proto

with the root as chit-chat