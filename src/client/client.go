package client

import (
	"encoding/json"
	"errors"
	"github.com/BernardSimon/gwdt/src/config"
	"github.com/BernardSimon/gwdt/utils"
	"github.com/levigross/grequests"
	"github.com/tidwall/gjson"
	"sort"
	"time"
)

type GwdtClient struct {
	Config config.GwdtConfig
}

func (c GwdtClient) getSign(timestamp int64, dataWrapper []byte, pager *config.GwdtPager, method string) (string, map[string]string, error) {
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
		value := utils.ToString(signParams[k])
		signFormat += k + value
		params[k] = value
	}
	signFormat += secret
	sign := utils.MD5(signFormat)
	params["signFormat"] = sign
	return sign, params, nil
}

func (c GwdtClient) Call(request *config.GwdtRequest) *config.GwdtResponse {
	var res config.GwdtResponse
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
