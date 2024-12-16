# Gwdt 旺店通旗舰版Go语言SDK

本SDK非官方SDK，为方便使用Go语言的开发者，按照旺店通官方文档，封装了签名及调用方法，使用本SDK可以根据业务需求直接调用相关方法。
请注意：调用旺店通API需要提前在旺店通开放平台申请API权限。

# Gwdt 特点
1. 支持旺店通直连和奇门自定义接口。
2. 允许通过中间件获取上下文并完成日志、限流等辅助业务逻辑。

## 1.安装方法：
### go get github.com/BernardSimon/gwdt@latest
## 2.旺店通直连参考示例代码
    // 实例化一个客户端
	wdtClient := gwdt.NewGwdtClient(gwdt.Config{
		Url:       "http://wdt.wangdian.cn/openapi",
		V:         "1.0",
		Sid:       "", //填入你的卖家账号
		AppKey:    "", //填入你的appkey，建议使用环境变量
		AppSecret: "", //填入你的appSecret，注意需要包含salt，建议使用环境变量
	})
    //实例化一个请求，如果不是分页请求，请将Pager置为nil
    //如果你有中间件，请定义一个中间件方法，并使用Client.Use(func(*gwdt.Context))定义中间件，具体请参考下文关于中间件的文档
	request := gwdt.Request{
		Method: "",
		Params: nil,
		Pager: &gwdt.Pager{
			PageSize: 100,
			PageNo:   0,
			CalcTotal:   true,
		},
	}
	//调用Call方法，得到一个Response实例
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
    请参考WdtError分析请求错误，其中Message为接口返回错误，RequestError为调用错误
## 3.奇门自定义参考示例代码
    // 实例化一个奇门客户端
	qimenClient := gwdt.NewGwdtQimenClient(gwdt.QimenConfig{
		QimenUrl:       "", //填入奇门地址
		QimenAppKey:    "", //填入你的奇门appkey，建议使用环境变量
		QimenAppSecret: "", //填入你的奇门appSecret，注意需要包含salt，建议使用环境变量
		Sid:            "", //填入你的卖家账号
		WdtAppKey:      "", //填入你的旺店通appkey，建议使用环境变量
		WdtAppSecret:   "", //填入你的旺店通appSecret，注意需要包含salt，建议使用环境变量
		TargetAppkey:   "", //填入旺店通目标appkey，建议使用环境变量
	})
    //如果你有中间件，请定义一个中间件方法，并使用QimenClient.Use(func(*gwdt.QimenContext))定义中间件，具体请参考下文关于中间件的文档
	qimenRequest := gwdt.QimenRequest{
		Method: "wdt.goods.apigoods.search",
		Pager: &gwdt.Pager{
			PageSize:  100,
			PageNo:    1, //请注意奇门接口的页码是从1开始
			CalcTotal: true,
		},
		Params: nil,
	}
    // 调用Call方法，得到一个Response实例
	qimenResponse := qimenClient.Call(&qimenRequest)
	if qimenResponse.Error != nil {
		panic(qimenResponse.Error)
	}
    //结果获取与直连方式相同
	println(qimenResponse.Data)
    请参考QimenError类型，分析请求错误
## 4.中间件使用说明
1. 中间件使用方法：gwdt的Client和QimenClient都支持使用中间件，需要实例化一个Client/QimenClient，然后使用Use方法定义一个中间件，中间件方法需要接收一个Context参数。
2. Context包含请求参数、返回结果、签名、时间戳等，方便开发者进行日志记录、限流等辅助业务逻辑，具体为Request和Response两个对象，调用Context.Next()方法后可以查看到Response的数据。
3. 请注意奇门调用需要使用QimenContext，两者不通用。
4. 请参考以下代码：
####
    // 定义一个中间件
    func wdtLog(c *gwdt.Context) {
    println("start")
    c.Next()
    println("end")
    }
    
    Client = gwdt.NewGwdtClient(gwdt.Config{
    Url:       "https://wdt.wangdian.cn/openapi",
    Sid:       "wdt_sid",
    AppKey:    "wdt_key",
    AppSecret: "wdt_secret",
    V:         "1.0",
    })
    Client.Use(*wdtLog)
    //本代码完成了一个简单的中间件使用示例，将在旺店通请求前打印start，请求结束后打印end
#### 奇门接口官方文档地址：https://open.wangdian.cn/qjb/open/guide?path=qjb_guide_qm_customize
#### 本代码问题可以联系: bernardziyi@gmail.com