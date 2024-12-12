package gwdt

import (
	"encoding/json"
	"errors"
	"github.com/BernardSimon/gwdt/gwdtUtils"
	"github.com/levigross/grequests"
	"github.com/tidwall/gjson"
	"sort"
	"strings"
	"time"
)

type Client struct {
	Config Config
}

func (c Client) getSign(timestamp int64, dataWrapper []byte, pager *Pager, method string) (string, map[string]string, error) {
	secret, salt, err := c.Config.GetSecret()
	if err != nil {
		return "", nil, err
	}
	var signParams = map[string]interface{}{
		"sid":       c.Config.Sid,
		"key":       c.Config.AppKey,
		"v":         c.Config.V,
		"method":    method,
		"salt":      salt,
		"timestamp": timestamp,
		"body":      dataWrapper,
	}
	if pager != nil {
		calcTotal := 0
		if pager.CalcTotal {
			calcTotal = 1
		}
		signParams["page_size"] = pager.PageSize
		signParams["page_no"] = pager.PageNo
		signParams["calc_total"] = calcTotal
	}
	keys := make([]string, 0, len(signParams))
	for k := range signParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	signFormat := secret
	params := make(map[string]string)
	for _, k := range keys {
		value := gwdtUtils.ToString(signParams[k])
		signFormat += k + value
		params[k] = value
	}
	signFormat += secret
	sign := gwdtUtils.MD5(signFormat)
	params["signFormat"] = sign
	return sign, params, nil
}

func (c Client) Call(request *Request) *Response {
	var res Response
	res.Request = request
	timestamp := time.Now().Unix() - 1325347200
	dataWrapper, err := json.Marshal([]interface{}{request.Params})
	if err != nil {
		res.Error = err
		return &res
	}
	var params map[string]string
	res.Sign, params, res.Error = c.getSign(timestamp, dataWrapper, request.Pager, request.Method)
	if res.Error != nil {
		return &res
	}
	resp, err := grequests.Post(c.Config.Url, &grequests.RequestOptions{JSON: dataWrapper, Params: params})
	if err != nil && resp == nil {
		res.Error = err
		return &res
	} else {
		status := gjson.Get(resp.String(), "status")
		if status.Int() != 0 {
			res.Status = int(status.Int())
			res.Error = errors.New(gjson.Get(resp.String(), "message").String())
			return &res
		}
		res.Status = 0
		res.Data = gjson.Get(resp.String(), "data").String()
		if request.Pager != nil && request.Pager.CalcTotal {
			res.TotalCount = gjson.Get(resp.String(), "total_count").Int()
		} else {
			res.TotalCount = 0
		}
	}
	return &res
}

type Config struct {
	Url       string `json:"url"` // 接口请求地址
	V         string `json:"v"`
	Sid       string `json:"sid"`       // 卖家账号
	AppKey    string `json:"appkey"`    // 旺店通旗舰版appkey
	AppSecret string `json:"appsecret"` // 旺店通旗舰版appsecret
}

func (c Config) GetSecret() (secret string, salt string, error error) {
	parts := strings.Split(c.AppSecret, ":")
	if len(parts) != 2 {
		return "", "", errors.New("invalid appsecret format")
	}
	secret = parts[0]
	salt = parts[1]
	return secret, salt, nil
}

type Pager struct {
	PageSize  int  // 每页数量
	PageNo    int  // 页码
	CalcTotal bool // 是否计算总条数
}

type Request struct {
	Method string
	Params map[string]interface{}
	Pager  *Pager
}

type Response struct {
	Request    *Request
	Status     int
	Error      error
	Timestamp  int64  // 按照旺店通规则的请求时间戳
	Sign       string // 按照旺店通规则的签名
	Data       string
	TotalCount int64
}

func (c Response) GetByte() []byte {
	return []byte(c.Data)
}
func (c Response) Get(key string) string {
	return gjson.Get(c.Data, key).String()
}
func (c Response) HasMore() bool {
	if c.Request.Pager == nil || !c.Request.Pager.CalcTotal {
		return false
	}
	return c.TotalCount > int64(c.Request.Pager.PageNo*c.Request.Pager.PageSize)
}

func NewGwdtClient(config Config) *Client {
	return &Client{Config: config}
}
