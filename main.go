package main

import (
	"github.com/BernardSimon/gwdt/src/client"
	"github.com/BernardSimon/gwdt/src/config"
)

type GwdtConfig = config.GwdtConfig
type GwdtClient = client.GwdtClient
type GwdtRequest = config.GwdtRequest
type GwdtResponse = config.GwdtResponse

func NewGwdtClient(config GwdtConfig) *GwdtClient {
	return &GwdtClient{Config: config}
}
