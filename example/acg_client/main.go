package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/vnotes/http2grpc-gateway/api/acgapi"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	var ctx = context.Background()
	r, _ := protojson.Marshal(&acgapi.AnimationRequest{Name: "東京喰種トーキョーグール", Timestamp: timestamppb.Now()})
	body := bytes.NewReader(r)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:9999/proxy", body)
	if err != nil {
		log.Fatalf("new request err %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 实际中一般是服务注册名，例如：acg/AcgService/Animation
	req.Header.Set("prism", "127.0.0.1:8888/AcgService/Animation")
	c := &http.Client{}
	response, err := c.Do(req)
	if err != nil {
		log.Fatalf("do request err %s", err)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("read err %s", err)
	}
	response.Body.Close()
	fmt.Printf("do request result %s\n", data)
}
