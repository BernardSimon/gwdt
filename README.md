# Gwdt 旺店通旗舰版Go语言SDK

本SDK非官方SDK，为方便使用Go语言的开发者，按照旺店通官方文档，封装了签名及调用方法，使用本SDK可以根据业务需求直接调用相关方法。
请注意：调用旺店通API需要提前在旺店通开放平台申请API权限。

## 1.安装方法：go get github.com/BernardSimon/gwdt
## 2.参考示例代码
    // 实例化一个客户端
	wdtClient := gwdt.NewGwdtClient(gwdt.Config{
		Url:       "http://wdt.wangdian.cn/openapi",
		V:         "1.0",
		Sid:       "", //填入你的卖家账号
		AppKey:    "", //填入你的appkey，建议使用环境变量
		AppSecret: "", //填入你的appSecret，注意需要包含salt，建议使用环境变量
	})
    //实例化一个请求，如果不是分页请求，请将Pager置为nil
	request := gwdt.Request{
		Method: "",
		Params: nil,
		Pager: &gwdt.Pager{
			PageSize: 100,
			PageNo:   0,
			CalcTotal:   true,
		},
	}
	//调用call方法，得到一个Response实例
	response := wdtClient.Call(&request)
    //处理返回结果，判断错误在前
	if response.Error != nil {
		panic(response.Error)
	}
    //Data是一个json的字符串，为旺店通返回的data字段内容
	println(response.Data)
    //为了方便记录和分析问题，我们在Response中增加了几个常用方法和变量
    1.内置请求指针 Response.Request
    2.计算的签名值 Response.Sign
    3.请求的时间戳(按照旺店通计算方法) Response.Timestamp
    4.请求的总条数 Response.TotalCount，请注意只有当分页请求且CalcTotal为true时，才返回该值
    5.GetByte方法，将返回结果转换为[]byte
    6.HasMore方法，判断是否还有更多数据