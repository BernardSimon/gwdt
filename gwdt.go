package gwdt

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// md5Sum 计算字符串的MD5值
func md5Sum(s string) string {
	hash := md5.New()
	hash.Write([]byte(s))
	return hex.EncodeToString(hash.Sum(nil))
}

// GwdtError 旺店通错误
type WdtError struct {
	Message      string `json:"message"`       // 接口错误信息
	RequestError error  `json:"request_error"` // 请求错误
}

func (e *WdtError) Error() string {
	if e.RequestError != nil {
		return fmt.Sprintf("WdtError: %s, RequestError: %v", e.Message, e.RequestError)
	}
	return fmt.Sprintf("WdtError: %s", e.Message)
}

// Pager 分页器
type Pager struct {
	PageSize  int  // 每页数量
	PageNo    int  // 页码
	CalcTotal bool // 是否计算总条数
}

// Request 请求参数
type Request struct {
	Method string
	Params interface{}
	Pager  *Pager
}

// Response 旺店通直连响应
type Response struct {
	Request    *Request  // 原始请求
	Status     int64     // 返回状态码，-1为请求失败，其他为接口返回状态码
	Error      *WdtError // 错误信息
	Timestamp  int64     // 按照旺店通规则的请求时间戳
	Sign       string    // 按照旺店通规则的签名
	Data       string    // 原始返回数据json字符串
	TotalCount int64     // 总条数，仅计算分页时使用
}

// GetByte 获取原始返回数据
func (c *Response) GetByte() []byte {
	return []byte(c.Data)
}

// Get 按键名获取原始返回数据
func (c *Response) Get(key string) string {
	var dataMap map[string]interface{}
	err := json.Unmarshal([]byte(c.Data), &dataMap)
	if err != nil {
		return ""
	}

	// Simple JSON path parsing for basic cases, can be extended for complex paths
	keys := strings.Split(key, ".")
	var current interface{} = dataMap
	for _, k := range keys {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[k]
		} else {
			return ""
		}
	}

	return fmt.Sprintf("%v", current)
}

// HasMore 判断是否还有更多数据，仅计算分页时使用
func (c *Response) HasMore() bool {
	if c.Request.Pager == nil || !c.Request.Pager.CalcTotal {
		return false
	}
	return c.TotalCount > int64((c.Request.Pager.PageNo+1)*c.Request.Pager.PageSize)
}

// Context 旺店通直连上下文管理器
type Context struct {
	Request     *Request
	Response    *Response
	Client      *Client
	middlewares []*func(ctx *Context)
	no          int
}

// Next 上下文跳转
func (c *Context) Next() {
	c.no += 1
	if c.no < len(c.middlewares) {
		nextFunc := *c.middlewares[c.no]
		nextFunc(c)
	}
}

// Config 旺店通直连配置
type Config struct {
	Url       string `json:"url"` // 接口请求地址
	V         string `json:"v"`
	Sid       string `json:"sid"`       // 卖家账号
	AppKey    string `json:"appkey"`    // 旺店通旗舰版appkey
	AppSecret string `json:"appsecret"` // 旺店通旗舰版appsecret
}

// getSecret 获取旺店通密钥
func (c *Config) getSecret() (secret string, salt string, error error) {
	parts := strings.Split(c.AppSecret, ":")
	if len(parts) != 2 {
		return "", "", errors.New("invalid appsecret format")
	}
	secret = parts[0]
	salt = parts[1]
	return secret, salt, nil
}

// Client 旺店通直连客户端
type Client struct {
	Config      Config
	middlewares []*func(*Context)
}

// NewGwdtClient 旺店通直连客户端构建函数
func NewGwdtClient(config Config) *Client {
	c := &Client{Config: config}
	c.Use(c.rq)
	return c
}

// Use 使用旺店通中间件
func (c *Client) Use(middleware func(ctx *Context)) {
	c.middlewares = append(c.middlewares, &middleware)
}

// getSign 获取旺店通请求签名
func (c *Client) getSign(timestamp int64, dataWrapper []byte, pager *Pager, method string) (string, map[string]string, error) {
	secret, salt, err := c.Config.getSecret()
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
	sign := md5Sum(signConn)
	params["sign"] = sign
	delete(params, "body")
	return sign, params, nil
}

// Call 执行请求方法，含中间件
func (c *Client) Call(request *Request) *Response {
	ctx := Context{
		Request:     request,
		Response:    nil,
		Client:      c,
		middlewares: c.middlewares,
		no:          0,
	}
	if len(c.middlewares) > 0 {
		nextFunc := *c.middlewares[0]
		nextFunc(&ctx)
	}
	return ctx.Response
}

// CallWithoutMiddleware 执行请求方法，不包含中间件
func (c *Client) CallWithoutMiddleware(request *Request) *Response {
	ctx := Context{
		Request: request,
	}
	c.rq(&ctx)
	return ctx.Response
}

// rq 请求底层方法
func (c *Client) rq(ctx *Context) {
	request := ctx.Request
	res := Response{
		Request:   request,
		Status:    -1,
		Timestamp: time.Now().Unix() - 1325347200,
	}
	dataWrapper := []byte("[{}]")
	if request.Params != nil {
		var err error
		// Marshal params to JSON first
		tempData, err := json.Marshal(request.Params)
		if err != nil {
			res.Error = &WdtError{
				RequestError: fmt.Errorf("failed to marshal request params: %w", err),
			}
			ctx.Response = &res
			return
		}

		// Check if the marshaled JSON is an array
		if len(tempData) > 0 && tempData[0] == '[' {
			dataWrapper = tempData
		} else {
			// If it's an object, wrap it in an array
			dataWrapper = []byte(fmt.Sprintf("[%s]", tempData))
		}
	}

	var err error
	var params map[string]string
	res.Sign, params, err = c.getSign(res.Timestamp, dataWrapper, request.Pager, request.Method)
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}

	// Build HTTP request
	httpReq, err := http.NewRequest("POST", c.Config.Url, strings.NewReader(string(dataWrapper)))
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	q := httpReq.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	httpReq.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(httpResp.Body)

	body, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}

	var rawResponse map[string]interface{}
	err = json.Unmarshal(body, &rawResponse)
	if err != nil {
		res.Error = &WdtError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}

	statusVal, ok := rawResponse["status"].(float64)
	if !ok || int64(statusVal) != 0 {
		res.Status = -1 // Default to -1 if status is not 0 or not found
		if ok {
			res.Status = int64(statusVal)
		}
		messageVal, _ := rawResponse["message"].(string)
		res.Error = &WdtError{
			Message: messageVal,
		}
		ctx.Response = &res
		return
	}

	res.Status = 0
	dataVal, ok := rawResponse["data"].(interface{})
	if ok {
		dataBytes, _ := json.Marshal(dataVal)
		res.Data = string(dataBytes)
	}

	if request.Pager != nil && request.Pager.CalcTotal {
		var dataMap map[string]interface{}
		err = json.Unmarshal([]byte(res.Data), &dataMap)
		if err != nil {
			res.Error = &WdtError{
				RequestError: err,
			}
			ctx.Response = &res
			return
		}
		if totalCountVal, ok := dataMap["total_count"].(float64); ok {
			res.TotalCount = int64(totalCountVal)
		} else {
			res.TotalCount = 0
		}
	} else {
		res.TotalCount = 0
	}
	ctx.Response = &res
	return
}

// 奇门客户端

// QimenConfig 奇门配置
type QimenConfig struct {
	QimenUrl       string `json:"qimen_url"`       // 奇门地址
	QimenAppKey    string `json:"qimen_appkey"`    // 奇门appkey
	QimenAppSecret string `json:"qimen_appsecret"` // 奇门appsecret
	Sid            string `json:"sid"`             // 卖家账号
	WdtAppKey      string `json:"wdt_appkey"`      // 旺店通旗舰版appkey
	WdtAppSecret   string `json:"wdt_appsecret"`   // 旺店通旗舰版appsecret
	TargetAppkey   string `json:"target_appkey"`   // 目标appkey
}

// getSecret 获取密钥与盐
func (c *QimenConfig) getSecret() (secret string, salt string, error error) {
	parts := strings.Split(c.WdtAppSecret, ":")
	if len(parts) != 2 {
		return "", "", errors.New("invalid appsecret format")
	}
	secret = parts[0]
	salt = parts[1]
	return secret, salt, nil
}

// QimenClient 奇门客户端
type QimenClient struct {
	Config      QimenConfig
	middlewares []*func(*QimenContext)
}

// NewGwdtQimenClient 奇门客户端构建函数
func NewGwdtQimenClient(qimenConfig QimenConfig) *QimenClient {
	c := &QimenClient{Config: qimenConfig}
	c.Use(c.rq)
	return c
}

// QimenRequest 奇门请求
type QimenRequest = Request

// QimenResponse 奇门响应
type QimenResponse struct {
	Request    *QimenRequest // 原始请求
	Status     int64         // 状态码，-1为请求失败，0为请求成功，1为返回错误
	Error      *QimenError   // 返回错误
	DateTime   string        // 按照旺店通规则的请求时间戳
	Sign       string        // 奇门签名
	WdtSign    string        // 按照旺店通规则的签名
	Data       string        // 返回数据json字符串
	TotalCount int64         // 分页查询返回的总记录数，仅当分页获取总数时返回
}
type QimenError struct {
	Flag         string // 奇门返回的错误标识
	RequestId    string // 奇门返回的请求id
	Code         string // 奇门返回的错误码
	Message      string // 奇门返回的错误信息
	SubCode      string // 奇门返回的子错误码
	SubMsg       string // 奇门返回的子错误信息
	RequestError error  // 原始请求错误
}

func (e *QimenError) Error() string {
	if e.RequestError != nil {
		return fmt.Sprintf("QimenError: Flag=%s, RequestId=%s, Code=%s, Message=%s, SubCode=%s, SubMsg=%s, RequestError: %v",
			e.Flag, e.RequestId, e.Code, e.Message, e.SubCode, e.SubMsg, e.RequestError)
	}
	return fmt.Sprintf("QimenError: Flag=%s, RequestId=%s, Code=%s, Message=%s, SubCode=%s, SubMsg=%s",
		e.Flag, e.RequestId, e.Code, e.Message, e.SubCode, e.SubMsg)
}

// GetByte 获取奇门返回数据
func (c *QimenResponse) GetByte() []byte {
	return []byte(c.Data)
}

// Get 按键获取奇门返回数据
func (c *QimenResponse) Get(key string) string {
	var dataMap map[string]interface{}
	err := json.Unmarshal([]byte(c.Data), &dataMap)
	if err != nil {
		return ""
	}

	// Simple JSON path parsing for basic cases, can be extended for complex paths
	keys := strings.Split(key, ".")
	var current interface{} = dataMap
	for _, k := range keys {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[k]
		} else {
			return ""
		}
	}

	return fmt.Sprintf("%v", current)
}

// HasMore 是否还有更多数据，仅分页且获取总数时返回
func (c *QimenResponse) HasMore() bool {
	if c.Request.Pager == nil || !c.Request.Pager.CalcTotal {
		return false
	}
	return c.TotalCount > int64((c.Request.Pager.PageNo)*c.Request.Pager.PageSize)
}

// getSortedParams 获取排序后的请求参数
func (c *QimenRequest) getSortedParams() ([]byte, error) {
	// 提取所有键
	mapParams, ok := c.Params.(map[string]interface{})
	if !ok {
		return nil, errors.New("params is not a map[string]interface{}")
	}
	keys := make([]string, 0, len(mapParams))
	for k := range mapParams {
		keys = append(keys, k)
	}
	// 对键进行排序
	sort.Strings(keys)
	// 按照排序后的键遍历 map
	sortedParams := make(map[string]interface{})
	for _, k := range keys {
		sortedParams[k] = mapParams[k]
	}
	// 将排序后的 map 序列化为 JSON 字符串
	jsonData, err := json.Marshal(sortedParams)
	if err != nil {
		return []byte(""), err
	}
	return jsonData, nil
}

// QimenContext 奇门上下文管理器
type QimenContext struct {
	Request     *QimenRequest
	Response    *QimenResponse
	Client      *QimenClient
	middlewares []*func(ctx *QimenContext)
	no          int
}

// Next 奇门上下文跳转
func (c *QimenContext) Next() {
	c.no += 1
	if c.no < len(c.middlewares) {
		nextFunc := *c.middlewares[c.no]
		nextFunc(c)
	}
}

// Use 添加奇门中间件
func (c *QimenClient) Use(middleware func(ctx *QimenContext)) {
	c.middlewares = append(c.middlewares, &middleware)
}

// getSign 获取奇门签名
func (c *QimenClient) getSign(timestamp string, dataWrapper []byte, pager *Pager, method string) (string, string, map[string]string, error) {
	_, wdtSalt, err := c.Config.getSecret()
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
	sign := strings.ToUpper(md5Sum(connString))
	params["sign"] = sign

	return sign, wdtSign, params, nil
}

// getWdtSign 获取旺店通签名
func (c *QimenClient) getWdtSign(datetime string, dataWrapper []byte, pager *Pager, method string) (string, error) {
	wdtSecret, wdtSalt, err := c.Config.getSecret()
	if err != nil {
		return "", err
	}
	params := map[string]string{
		"method":           method,
		"datetime":         datetime,
		"wdt3_customer_id": c.Config.Sid,
		"wdt_salt":         wdtSalt,
		"wdt_appkey":       c.Config.WdtAppKey,
		"params":           string(dataWrapper),
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
	return md5Sum(connString), nil
}

// Call 执行奇门接口请求
func (c *QimenClient) Call(request *QimenRequest) *QimenResponse {
	ctx := QimenContext{
		Request:     request,
		Response:    nil,
		Client:      c,
		middlewares: c.middlewares,
		no:          0,
	}
	if len(c.middlewares) > 0 {
		nextFunc := *c.middlewares[0]
		nextFunc(&ctx)
	}
	return ctx.Response
}

// CallWithoutMiddleware 执行奇门接口请求，不执行中间件
func (c *QimenClient) CallWithoutMiddleware(request *QimenRequest) *QimenResponse {
	ctx := QimenContext{
		Request:  request,
		Response: nil,
	}
	c.rq(&ctx)
	return ctx.Response
}

// rq 奇门底层请求方法
func (c *QimenClient) rq(ctx *QimenContext) {
	request := ctx.Request
	res := QimenResponse{
		Status:   -1,
		Request:  request,
		DateTime: time.Now().Format("2006-01-02 15:04:05"),
	}
	var err error
	dataWrapper := []byte("{}")
	if request.Params != nil {
		dataWrapper, err = json.Marshal(request.Params)
		if err != nil {
			res.Error = &QimenError{
				Message:      "Failed to marshal params",
				RequestError: err,
			}
			ctx.Response = &res
			return
		}
	}

	var params map[string]string
	res.Sign, res.WdtSign, params, err = c.getSign(res.DateTime, dataWrapper, request.Pager, request.Method)
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}

	// Build HTTP request
	httpReq, err := http.NewRequest("GET", c.Config.QimenUrl, nil)
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}

	q := httpReq.URL.Query()
	for k, v := range params {
		q.Add(k, v)
	}
	httpReq.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()

	}(httpResp.Body)

	body, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}

	var rawResponse map[string]interface{}
	err = json.Unmarshal(body, &rawResponse)
	if err != nil {
		res.Error = &QimenError{
			RequestError: err,
		}
		ctx.Response = &res
		return
	}

	responseMap, ok := rawResponse["response"].(map[string]interface{})
	if !ok {
		res.Error = &QimenError{
			RequestError: errors.New("invalid response format: missing 'response' key"),
		}
		ctx.Response = &res
		return
	}

	flag, _ := responseMap["flag"].(string)
	if flag == "failure" {
		res.Status = 1
		res.Error = &QimenError{
			Flag:         flag,
			RequestId:    fmt.Sprintf("%v", responseMap["request_id"]),
			Code:         fmt.Sprintf("%v", responseMap["code"]),
			Message:      fmt.Sprintf("%v", responseMap["message"]),
			SubCode:      fmt.Sprintf("%v", responseMap["sub_code"]),
			SubMsg:       fmt.Sprintf("%v", responseMap["sub_message"]),
			RequestError: nil,
		}
		ctx.Response = &res
		return
	}
	res.Status = 0
	dataVal, ok := responseMap["data"].(interface{})
	if ok {
		dataBytes, _ := json.Marshal(dataVal)
		res.Data = string(dataBytes)
	}

	if request.Pager != nil && request.Pager.CalcTotal {
		var dataMap map[string]interface{}
		err = json.Unmarshal([]byte(res.Data), &dataMap)
		if err != nil {
			res.Error = &QimenError{
				RequestError: err,
			}
			ctx.Response = &res
			return
		}
		if totalCountVal, ok := dataMap["total_count"].(float64); ok {
			res.TotalCount = int64(totalCountVal)
		} else {
			res.TotalCount = 0
		}
	} else {
		res.TotalCount = 0
	}
	ctx.Response = &res
	return
}
