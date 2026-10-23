Handin 2 is for help when coding (will be removed)

chit chat is the actual code


## Regenerate chitchat_grpc.pb.go and chitchat.pb.go when changing chitchat.proto

When changing the chitchat proto remember to regenerate the two files using:

protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/chitchat.proto

with the root as chit-chat

## Usefull commands
Ensures that your project's go.mod and go.sum files match the actual source code in your project
```
go mod tidy
```

Test that the GO Packages compiles.
Should say something like:
...
?  github.com/YOURUSERNAME/chit-chat/client           [no test files]
?  github.com/YOURUSERNAME/chit-chat/chit-chat/grpc     [no test files]
?  github.com/YOURUSERNAME/chit-chat/chit-chat/server   [no test files]
...
```
go test ./...
```

Starts the server
```
go run ./server
```