package gwdt

import (
	"github.com/BernardSimon/gwdt/src/client"
	"github.com/BernardSimon/gwdt/src/config"
)

type Config = config.GwdtConfig
type Client = client.GwdtClient
type Request = config.GwdtRequest
type Response = config.GwdtResponse

func NewGwdtClient(config Config) *Client {
	return &Client{Config: config}
}
