package service

import "github.com/alldev-run/golang-gin-rpc/api/http-gateway/internal/model"

type HelloService struct{}

func NewHelloService() *HelloService {
	return &HelloService{}
}

func (s *HelloService) Hello() model.HelloResponse {
	return model.HelloResponse{Message: "Hello World"}
}
