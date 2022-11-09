package ui_config

type Config struct { //nolint:maligned
	LogDebug bool `hcl:"log_debug"`
	Front    struct {
		MsgError       string `hcl:"msg_error"`
		MsgMenuError   string `hcl:"msg_menu_error"`
		MsgStateBroken string `hcl:"msg_broken"`
		MsgBrokenL1    string `hcl:"msg_broken_l1"`
		MsgBrokenL2    string `hcl:"msg_broken_l2"`
		MsgStateLocked string `hcl:"msg_locked"`
		MsgStateIntro  string `hcl:"msg_intro"`
		MsgWait        string `hcl:"msg_wait"`
		MsgWaterTemp   string `hcl:"msg_water_temp"`

		MsgMenuCodeEmpty          string `hcl:"msg_menu_code_empty"`
		MsgMenuCodeInvalid        string `hcl:"msg_menu_code_invalid"`
		MsgMenuInsufficientCredit string `hcl:"msg_menu_insufficient_credit_l1"`
		// MsgMenuInsufficientCreditL2 string `hcl:"msg_menu_insufficient_credit_l2"`
		MsgMenuNotAvailable string `hcl:"msg_menu_not_available"`

		MsgCream   string `hcl:"msg_cream"`
		MsgSugar   string `hcl:"msg_sugar"`
		MsgCredit  string `hcl:"msg_credit"`
		MsgMaking1 string `hcl:"msg_making1"`
		MsgMaking2 string `hcl:"msg_making2"`

		MsgInputCode string `hcl:"msg_input_code"`
		MsgPrice     string `hcl:"msg_price"`

		MsgRemotePay        string `hcl:"msg_remote_pay"`
		MsgRemotePayRequest string `hcl:"msg_remote_pay_request"`

		MsgNoNetwork string `hcl:"msg_no_network"`

		ResetTimeoutSec int    `hcl:"reset_sec"`
		PicBoot         string `hcl:"pic_boot"`
		PicClient       string `hcl:"pic_client"`
		PicIdle         string `hcl:"pic_idle"`
		PicMake         string `hcl:"pic_make"`
		PicBroken       string `hcl:"pic_broken"`
		PicQRPayError   string `hcl:"pic_QR_pay_error"`
		PicPayReject    string `hcl:"pic_pay_reject"`

		LightShedule string `hcl:"light_sheduler"`
	}

	Service struct {
		Auth struct {
			Enable    bool     `hcl:"enable"`
			Passwords []string `hcl:"passwords"`
		}
		MsgAuth         string `hcl:"msg_auth"`
		ResetTimeoutSec int    `hcl:"reset_sec"`
		Tests           []struct {
			Name     string `hcl:"name,key"`
			Scenario string `hcl:"scenario"`
		} `hcl:"test"`
	}
}
