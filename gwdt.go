package gwdt

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BernardSimon/gwdt/gwdtUtils"
	"github.com/levigross/grequests"
	"github.com/tidwall/gjson"
	"sort"
	"strconv"
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
	var params = map[string]string{
		"sid":       c.Config.Sid,
		"key":       c.Config.AppKey,
		"v":         c.Config.V,
		"method":    method,
		"salt":      salt,
		"timestamp": strconv.FormatInt(timestamp, 10),
		"body":      string(dataWrapper),
	}
	if pager != nil {
		if pager.CalcTotal {
			params["calc_total"] = "1"
		} else {
			params["calc_total"] = "0"
		}
		params["page_size"] = strconv.Itoa(pager.PageSize)
		params["page_no"] = strconv.Itoa(pager.PageNo)
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	signConn := secret
	for _, k := range keys {
		value := params[k]
		signConn += k + value
	}
	signConn += secret
	sign := gwdtUtils.MD5(signConn)
	params["sign"] = sign
	delete(params, "body")
	return sign, params, nil
}

func (c Client) Call(request *Request) *Response {
	res := Response{
		Request:   request,
		Status:    -1,
		Timestamp: time.Now().Unix() - 1325347200,
	}
	dataWrapper, err := json.Marshal([]interface{}{request.Params})
	if request.Params == nil {
		dataWrapper = []byte("[{}]")
	}
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		return &res
	}
	var params map[string]string
	res.Sign, params, err = c.getSign(res.Timestamp, dataWrapper, request.Pager, request.Method)
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		return &res
	}
	resp, err := grequests.Post(c.Config.Url, &grequests.RequestOptions{JSON: dataWrapper, Params: params})
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		return &res
	} else if resp == nil {
		res.Error = &WdtError{
			RequestError: errors.New("request failed"),
		}
		return &res
	} else {
		status := gjson.Get(resp.String(), "status")
		if status.Int() != 0 {
			res.Status = status.Int()
			res.Error = &WdtError{
				Message: gjson.Get(resp.String(), "message").String(),
			}
			return &res
		}
		res.Status = 0
		res.Data = gjson.Get(resp.String(), "data").String()
		if request.Pager != nil && request.Pager.CalcTotal {
			res.TotalCount = gjson.Get(res.Data, "total_count").Int()
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
type WdtError struct {
	Message      string `json:"Message"`
	RequestError error  `json:"request_error"`
}
type Response struct {
	Request    *Request
	Status     int64
	Error      *WdtError
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
	return c.TotalCount > int64((c.Request.Pager.PageNo+1)*c.Request.Pager.PageSize)
}

func NewGwdtClient(config Config) *Client {
	return &Client{Config: config}
}
func NewGwdtQimenClient(qimenConfig QimenConfig) *QimenClient {
	return &QimenClient{Config: qimenConfig}
}

type QimenConfig struct {
	QimenUrl       string `json:"qimen_url"`
	QimenAppKey    string `json:"qimen_appkey"`
	QimenAppSecret string `json:"qimen_appsecret"`
	Sid            string `json:"sid"`           // 卖家账号
	WdtAppKey      string `json:"wdt_appkey"`    // 旺店通旗舰版appkey
	WdtAppSecret   string `json:"wdt_appsecret"` // 旺店通旗舰版appsecret
	TargetAppkey   string `json:"target_appkey"`
}

func (c QimenConfig) GetSecret() (secret string, salt string, error error) {
	parts := strings.Split(c.WdtAppSecret, ":")
	if len(parts) != 2 {
		return "", "", errors.New("invalid appsecret format")
	}
	secret = parts[0]
	salt = parts[1]
	return secret, salt, nil
}

type QimenClient struct {
	Config QimenConfig
}

type QimenRequest = Request

func (c QimenRequest) GetSortedParams() ([]byte, error) {
	// 提取所有键
	keys := make([]string, 0, len(c.Params))
	for k := range c.Params {
		keys = append(keys, k)
	}
	// 对键进行排序
	sort.Strings(keys)
	// 按照排序后的键遍历 map
	sortedParams := make(map[string]interface{})
	for _, k := range keys {
		sortedParams[k] = c.Params[k]
	}
	// 将排序后的 map 序列化为 JSON 字符串
	jsonData, err := json.Marshal(sortedParams)
	if err != nil {
		return []byte(""), err
	}
	return jsonData, nil
}

type QimenError struct {
	Flag         string
	RequestId    string
	Code         string
	Message      string
	SubCode      string
	SubMsg       string
	RequestError error
}

type QimenResponse struct {
	Request    *QimenRequest
	Status     int64
	Error      *QimenError
	DateTime   string // 按照旺店通规则的请求时间戳
	Sign       string
	WdtSign    string // 按照旺店通规则的签名
	Data       string
	TotalCount int64
}

func (c QimenClient) getSign(timestamp string, dataWrapper []byte, pager *Pager, method string) (string, string, map[string]string, error) {
	_, wdtSalt, err := c.Config.GetSecret()
	if err != nil {
		return "", "", nil, err
	}
	wdtSign, err := c.getWdtSign(timestamp, dataWrapper, pager, method)
	if err != nil {
		return "", "", nil, err
	}
	params := map[string]string{
		"app_key":          c.Config.QimenAppKey,
		"method":           method,
		"format":           "json",
		"v":                "2.0",
		"sign_method":      "md5",
		"target_app_key":   c.Config.TargetAppkey,
		"wdt3_customer_id": c.Config.Sid,
		"timestamp":        timestamp,
		"datetime":         timestamp,
		"wdt_salt":         wdtSalt,
		"wdt_appkey":       c.Config.WdtAppKey,
		"params":           string(dataWrapper),
		"wdt_sign":         wdtSign,
	}
	if pager != nil {
		if pager.CalcTotal {
			params["calc_total"] = "1"
		} else {
			params["calc_total"] = "0"
		}
		params["pager"] = fmt.Sprintf(`{"page_no":%d,"page_size":%d}`, pager.PageNo, pager.PageSize)
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	connString := c.Config.QimenAppSecret
	for _, k := range keys {
		value := params[k]
		connString += k + value
	}
	connString += c.Config.QimenAppSecret
	sign := strings.ToUpper(gwdtUtils.MD5(connString))
	params["sign"] = sign
	return sign, wdtSign, params, nil
}

func (c QimenClient) getWdtSign(datetime string, dataWrapper []byte, pager *Pager, method string) (string, error) {
	wdtSecret, wdtSalt, err := c.Config.GetSecret()
	if err != nil {
		return "", err
	}
	params := map[string]string{
		"method":           method,
		"datetime":         datetime,
		"params":           string(dataWrapper),
		"wdt3_customer_id": c.Config.Sid,
		"wdt_salt":         wdtSalt,
		"wdt_appkey":       c.Config.WdtAppKey,
	}
	if pager != nil {
		if pager.CalcTotal {
			params["calc_total"] = "1"
		} else {
			params["calc_total"] = "0"
		}
		params["pager"] = fmt.Sprintf(`{"page_no":%d,"page_size":%d}`, pager.PageNo, pager.PageSize)

	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	connString := wdtSecret
	for _, k := range keys {
		connString += k + params[k]
	}
	connString += wdtSecret
	return gwdtUtils.MD5(connString), nil
}
func (c QimenClient) Call(request *QimenRequest) *QimenResponse {
	res := QimenResponse{
		Status:   -1,
		Request:  request,
		DateTime: time.Now().Format("2006-01-02 15:04:05"),
	}
	var err error
	var dataWrapper []byte
	if request.Params == nil {
		dataWrapper = []byte("{}")
	} else {
		dataWrapper, err = request.GetSortedParams()
		if err != nil {
			res.Error = &QimenError{
				Message:      "Failed to marshal params",
				RequestError: err,
			}
			return &res
		}
	}
	var params map[string]string
	res.Sign, res.WdtSign, params, err = c.getSign(res.DateTime, dataWrapper, request.Pager, request.Method)
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		return &res
	}

	resp, err := grequests.Get(c.Config.QimenUrl, &grequests.RequestOptions{Params: params})
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		return &res
	} else if resp == nil {
		res.Error = &QimenError{
			RequestError: errors.New("request failed"),
		}
		return &res
	}

	var responseMap map[string]interface{}
	err = json.Unmarshal([]byte(resp.String()), &responseMap)
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		return &res
	}

	flag := gjson.Get(resp.String(), "response.flag").String()
	if flag == "failure" {
		res.Status = 1
		res.Error = &QimenError{
			Flag:         flag,
			RequestId:    gjson.Get(resp.String(), "response.request_id").String(),
			Code:         gjson.Get(resp.String(), "response.code").String(),
			Message:      gjson.Get(resp.String(), "response.message").String(),
			SubCode:      gjson.Get(resp.String(), "response.sub_code").String(),
			SubMsg:       gjson.Get(resp.String(), "response.sub_msg").String(),
			RequestError: nil,
		}
		return &res
	}
	res.Status = 0
	res.Data = gjson.Get(resp.String(), "response.data").String()
	if request.Pager != nil {
		if request.Pager.CalcTotal {
			res.TotalCount = gjson.Get(res.Data, "response.total_count").Int()
		}
	} else {
		res.TotalCount = 0
	}
	return &res
}
