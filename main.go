package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/vnotes/http2grpc-gateway/proxyservice"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	proxyKey = "prism"
)

var (
	umo = &protojson.UnmarshalOptions{DiscardUnknown: true}
	mo  = &protojson.MarshalOptions{}
)

type Respoonse struct {
	Code    int
	Message string
}

// can hard code
func NewSySErr() []byte {
	data, _ := json.Marshal(&Respoonse{Code: -1, Message: "system error"})
	return data
}

func NewResponse(code int, message string) []byte {
	data, _ := json.Marshal(&Respoonse{Code: code, Message: message})
	return data
}

func main() {
	mux := http.NewServeMux()
	s := &http.Server{
		Addr:              ":9999",
		Handler:           mux,
		ReadHeaderTimeout: time.Minute,
	}

	mux.HandleFunc("/proxy", func(w http.ResponseWriter, r *http.Request) {
		// demo: host:port/service/method
		proxyValue := r.Header.Get(proxyKey)
		r.Header.Del(proxyKey)
		proxyInfo := strings.Split(proxyValue, "/")
		if len(proxyInfo) != 3 {
			fmt.Println("request type invalid")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(NewSySErr())
			return
		}
		invokeInfo := proxyservice.NewInvokeTarget(proxyInfo[0], proxyInfo[1], proxyInfo[2])
		method := proxyservice.XO.ByInvoke(invokeInfo)
		if method == nil {
			fmt.Println("get nil method")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(NewSySErr())
			return
		}
		var ctx = r.Context()
		body, _ := io.ReadAll(r.Body)
		request := dynamicpb.NewMessage(method.Args)
		response := dynamicpb.NewMessage(method.Reply)
		if err := umo.Unmarshal(body, request); err != nil {
			fmt.Printf("unmarshal err %s\n", err)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(NewSySErr())
			return
		}
		conn := proxyservice.XO.Client(invokeInfo.Host)
		if conn == nil {
			fmt.Println("get nil conn")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(NewSySErr())
			return
		}
		err := conn.Invoke(ctx, method.FullName, request, response)
		if err != nil {
			fmt.Printf("invoke err %s\n", err)
			value, ok := status.FromError(err)
			if ok {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(NewResponse(int(value.Code()), value.Message()))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(NewSySErr())
			return
		}
		data, _ := mo.Marshal(response)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	fmt.Printf("listening server %d\n", 9999)
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("listening err %s", err)
	}
}
