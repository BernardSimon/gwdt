package main

import (
	"gwdt/src/client"
	"gwdt/src/config"
)

type GwdtConfig = config.GwdtConfig
type GwdtClient = client.GwdtClient
type GwdtRequest = config.GwdtRequest
type GwdtResponse = config.GwdtResponse

func NewGwdtClient(config GwdtConfig) *GwdtClient {
	return &GwdtClient{Config: config}
}
