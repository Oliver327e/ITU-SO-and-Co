### Regenerate chitchat_grpc.pb.go and chitchat.pb.go when changing chitchat.proto

When changing the chitchat proto remember to regenerate the two files using the below command in the terminal with the root as chit-chat

protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/chitchat.proto

## Usefull commands
Ensures that your project's go.mod and go.sum files match the actual source code in your project
```
go mod tidy
```

Test that the GO Packages compiles.
Should say something like:

?  github.com/YOURUSERNAME/chit-chat/client           [no test files] \
?  github.com/YOURUSERNAME/chit-chat/chit-chat/grpc     [no test files] \
?  github.com/YOURUSERNAME/chit-chat/chit-chat/server   [no test files] \

```
go test ./...
```

Starts the server
```
go run ./server
```
