package main

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/vnotes/http2grpc-gateway/api/genproto/acg/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	*pb.UnimplementedAcgServiceServer
}

func (*server) Animation(ctx context.Context, in *pb.AnimationRequest) (*pb.AnimationResponse, error) {
	return &pb.AnimationResponse{Message: fmt.Sprintf("I like %s", in.Name)}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatalf("listen network error %s", err)
	}
	s := grpc.NewServer()
	pb.RegisterAcgServiceServer(s, &server{})
	reflection.Register(s)
	fmt.Println("listening server 8888")
	if err = s.Serve(lis); err != nil {
		log.Fatalf("failed to server %s", err)
	}
}
