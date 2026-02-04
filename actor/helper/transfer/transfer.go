package transfer

type DoActRecordListType int
type DoActApiType int
type DoActTransferType int

const (
	//账户划转记录及list
	DoActLGetWithdrawRecord DoActRecordListType = 1 + iota
	DoActLGetDepositRecord
	DoActLGetTransferRecord
	DoActLGetSubAccountList
	DoActLGetHFList
	DoActLGetBalance
	DoActLGetPosition

	//Api相关类型
	DoActACreateSpotSubApi DoActApiType = 1 + iota
	DoActACreateFutureApi
	DoActAGetSpotApiList
	DoActAGetFutureApiList
	DoActAModifySpotApi
	DoActAModifyFutureApi
	DoActADeleteSpotApi
	DoActADeleteFutureApi

	//划转类型
	DoActTMainToSubFuture DoActTransferType = 1 + iota
	DoActTSubFutureToMain
	DoActTMainToSubSpot
	DoActTSubSpotToMain
	DoActTMainSpotToFuture //主账户现货到主账户合约
	DoActTMainFutureToSpot //主账户合约到主账户现货
	DoActTMainSpotToTrade  //ku Specialty
	DoActTMainSpotToHF     //ku Specialty
	DoActTHFToMainSpot     //ku Specialty
	DoActTTradeToHF        //ku Specialty
)

type Parameter struct {
	AccountId   string `json:"accountId"`
	ApiKey      string `json:"apiKey"`
	Secret      string `json:"apiSecret"`
	Passphrase  string `json:"mainPassphrase"`
	SubPass     string `json:"subPassphrase"`
	Amount      string `json:"amount"`
	AccountType string `json:"accountType"`
	SubName     string `json:"subname"`
	Remarks     string `json:"remark"`
	IpWhitelist string `json:"ip"`
}

type TransferParams struct {
	SubAcctName       string  // 子账户名称
	ToSubAcctName     string  // 目标子账户名称
	SubAcctUid        string  // 子账户uid
	ToSubAcctUid      string  // 转入目标子账户uid
	SubAcctSpecialId  string  // 特殊ID（仅针对部分所）
	MainAcctUid       string  // 主账户uid
	ToMainAcctUid     string  // 转出主账户uid
	Amount            float64 // 转账金额
	TransferType      string  // 转账类型
	TransferDirection string  // 母子账户转账方向
	Source            string  // 发起划转的类型
	Target            string  // 划转的目标类型
	Asset             string  // 划转资产
}

type TransferAllDirectionParams struct {
	fromSymbol   string  // 必须要发送，当类型为 ISOLATEDMARGIN_MARGIN 和 ISOLATEDMARGIN_ISOLATEDMARGIN
	toSymbol     string  // 必须要发送，当类型为 MARGIN_ISOLATEDMARGIN 和 ISOLATEDMARGIN_ISOLATEDMARGIN
	Amount       float64 // 转账金额
	TransferType string  // 转账类型
	Asset        string  // 划转资产
}

type AccountInfo struct {
	SubUid     string `json:"uid"`        //子账户uid
	SubName    string `json:"subname"`    //子账户名称
	Note       string `json:"note"`       //备注
	Permission string `json:"permission"` //权限
	Balance    string `json:"balance"`    //资产

	Assert        float64 `json:"assert"`
	MarginAccount float64 `json:"marginAccount"` //杠杆  合约

	MainAccount    float64 `json:"mainAccount"`
	TradeAccount   float64 `json:"tradeAccount"`   //现货
	TradeHFAccount float64 `json:"tradeHFAccount"` //现货高频
}

// 创建子账户参数
type CreateSubAcctParams struct {
	UserList []map[string]interface{}
	Password string
	SubName  string
	Note     string
}

type APIOperateParams struct {
	UID      string // 子账户的UID
	Note     string // 备注   长度不能超过 50 个字符
	IP       string // IP白名单
	RemoveIP string // 移除IP白名单
	APIKey   string // apiKey
	Password string // 子账户密码
	SubName  string // 子账户名称
	Cover    bool
}

type WithDraw struct {
	Coin        string  `json:"coin"`      //提币或充币币种
	Chain       string  `json:"chain"`     //提币网络
	Amount      float64 `json:"amount"`    //数量
	ToAddress   string  `json:"toaddress"` //提币地址
	Withdraw    string  `json:"withdraw"`  //类型 提币
	Fee         string  `json:"fee"`       //手续费
	Id          string  `json:"id"`        //订单id
	CreatedTime string  `json:"createdTime"`
	FromType    string  `json:"fromType"`   //转出账户类型
	ToType      string  `json:"toType"`     //转入账户类型
	FromSymbol  string  `json:"fromSymbol"` //转出账户交易对
}

type Api struct {
	SubName    string   `json:"subName"`    //子账户名称
	Subuid     string   `json:"uid"`        //子账户uid
	Key        string   `json:"apiKey"`     //子账户key
	Secret     string   `json:"apiSecret"`  //子账户secret
	Passphrase string   `json:"passphrase"` //子账户密码
	Remark     string   `json:"remark"`     //备注
	Permission string   `json:"permission"` //权限
	Ip         string   `json:"ip"`
	IPs        []string `json:"ips"`
	ApiID      int
}

type Pnl struct {
	Symbol        string `json:"symbol"`   //币种
	HoldSide      string `json:"holdSide"` //
	Pnl           string `json:"pnl"`      //
	NetProfit     string `json:"netProfit"`
	Utime         string `json:"utime"` //
	Ctime         string `json:"ctime"`
	CloseTotalPos string `json:"closeTotalPos"` //
	OpenTotalPos  string `json:"openTotalPos"`  //
	OpenFee       string `json:"openFee"`
	CloseFee      string `json:"closeFee"`     //
	OpenAvgPrice  string `json:"openAvgPrice"` //
	CloseAvgPrice string `json:"closeAvgPrice"`
	EndId         string
}

type Response struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

type GetBalanceParams struct {
	SubAcctUid  string
	SubAcctName string // 子账户名称
	AccountType string
}

type GetMainBalanceParams struct {
	AccountType string
}

type GetWithDrawHistoryParams struct {
	Start   int64
	End     int64
	Account string
}
type WithDrawHistory struct {
	Platform     string //交易所平台
	Account      string //划出账户名字
	ApplyTime    int64  //需播报提币时间
	CompleteTime int64  //提现完成

	Amount   float64 //数量
	Address  string  //目标地址
	Currency string  //币种

	OrderID string //唯一识别
}
