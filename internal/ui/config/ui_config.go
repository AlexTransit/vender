package ui_config

import (
	"github.com/AlexTransit/vender/currency"
	"github.com/AlexTransit/vender/internal/menu/menu_config"
)

type Config struct { //nolint:maligned
	LogDebug bool          `hcl:"log_debug,optional"`
	Front    FrontStruct   `hcl:"front,block"`
	Service  ServiceStruct `hcl:"service,block"`
}

type FrontStruct struct {
	MsgMenuError string `hcl:"msg_menu_error"`
	MsgWait      string `hcl:"msg_wait"`
	MsgWaterTemp string `hcl:"msg_water_temp"`

	MsgMenuCodeEmpty            string `hcl:"msg_menu_code_empty"`
	MsgMenuCodeInvalid          string `hcl:"msg_menu_code_invalid"`
	MsgMenuInsufficientCreditL1 string `hcl:"msg_menu_insufficient_credit_l1"` // Мало денег
	MsgMenuInsufficientCreditL2 string `hcl:"msg_menu_insufficient_credit_l2"` // дали:%s нужно:%s
	MsgMenuNotAvailable         string `hcl:"msg_menu_not_available"`          //"Not available" // "Не доступен"

	MsgCream  string `hcl:"msg_cream"`
	MsgSugar  string `hcl:"msg_sugar"`
	MsgCredit string `hcl:"msg_credit"`

	MsgInputCode string `hcl:"msg_input_code"`
	MsgPrice     string `hcl:"msg_price"`

	MsgRemotePay        string `hcl:"msg_remote_pay"`
	MsgRemotePayRequest string `hcl:"msg_remote_pay_request"` // "QR request sent" // "QR запрос отправлен"
	MsgRemotePayReject  string `hcl:"msg_remote_pay_reject"`  // "Bank refused :(" // "Банк отказал :("

	MsgNoNetwork string `hcl:"msg_no_network"`

	ResetTimeoutSec int    `hcl:"reset_sec"`
	PicQRPayError   string `hcl:"pic_QR_pay_error"`
	PicPayReject    string `hcl:"pic_pay_reject"`

	LightShedule string `hcl:"light_sheduler"`
}

type ServiceStruct struct {
	ResetTimeoutSec int           `hcl:"reset_sec"`
	XXX_Tests       []TestsStruct `hcl:"test,block"`
	Tests           map[string]TestsStruct
}

type TestsStruct struct {
	Name     string `hcl:"name,label"`
	Scenario string `hcl:"scenario"`
}

type UIUser struct {
	QrText             string
	UiState            uint32
	Lock               bool
	DirtyMoney         currency.Amount
	KeyboardReadEnable bool
	menu_config.UIMenuStruct
}
