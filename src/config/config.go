package config

import (
	"errors"
	"github.com/tidwall/gjson"
	"strings"
)

type GwdtConfig struct {
	Url       string `json:"url"` // 接口请求地址
	V         string `json:"v"`
	Sid       string `json:"sid"`       // 卖家账号
	AppKey    string `json:"appkey"`    // 旺店通旗舰版appkey
	AppSecret string `json:"appsecret"` // 旺店通旗舰版appsecret
}

func (c GwdtConfig) GetSecret() (secret string, salt string, error error) {
	parts := strings.Split(c.AppSecret, ":")
	if len(parts) != 2 {
		return "", "", errors.New("invalid appsecret format")
	}
	secret = parts[0]
	salt = parts[1]
	return secret, salt, nil
}

type GwdtPager struct {
	PageSize  int  // 每页数量
	PageNo    int  // 页码
	CalcTotal bool // 是否计算总条数
}

type GwdtRequest struct {
	Method string
	Params map[string]interface{}
	Pager  *GwdtPager
}

type GwdtResponse struct {
	Request    *GwdtRequest
	Status     int
	Error      error
	Timestamp  int64  // 按照旺店通规则的请求时间戳
	Sign       string // 按照旺店通规则的签名
	Data       string
	TotalCount int64
}

func (c GwdtResponse) GetByte() []byte {
	return []byte(c.Data)
}
func (c GwdtResponse) Get(key string) string {
	return gjson.Get(c.Data, key).String()
}
func (c GwdtResponse) HaveMore() bool {
	if c.Request.Pager == nil || !c.Request.Pager.CalcTotal {
		return false
	}
	return c.TotalCount > int64(c.Request.Pager.PageNo*c.Request.Pager.PageSize)
}
