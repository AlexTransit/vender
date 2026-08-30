package ui_test

// minimalTestConfig — минимальный HCL конфиг для тестов UI.
// Не включает engine и inventory — добавляй их через testConfigWith().
const minimalTestConfig = `
money {
	scale        = 100
	credit_max   = 100
	minimal_bill = 0
	maximum_bill = 1000
}
hardware {
	display { framebuffer = "" }
	hd44780 {
		enable       = false
		pin_chip     = ""
		width        = 16
		blink        = false
		cursor       = false
		scroll_delay = 0
		pinmap {
			rs = "0"
			rw = "0"
			e  = "0"
			d4 = "0"
			d5 = "0"
			d6 = "0"
			d7 = "0"
		}
	}
	input {
		evend_keyboard {}
	}
	mdb {
		uart_driver = "dummy"
		bill { scaling_factor = 0 }
		coin {}
	}
	evend {
		espresso {}
		valve { temperature_hot = 0 }
	}
	mega {}
}
tele {}
ui {
	front {
		reset_sec                        = 5
		msg_menu_error                   = "error"
		msg_wait                         = "please wait"
		msg_water_temp                   = "temp:%d"
		msg_menu_code_empty              = "enter code"
		msg_menu_code_invalid            = "bad code"
		msg_menu_insufficient_credit_l1  = "not enough"
		msg_menu_insufficient_credit_l2  = "need more"
		msg_menu_not_available           = "not available"
		msg_cream                        = "cream"
		msg_sugar                        = "sugar"
		msg_credit                       = "Credit:"
		msg_input_code                   = "Code:%s"
		msg_price                        = "price:%s"
		msg_remote_pay                   = "QR"
		msg_remote_pay_request           = "QR request"
		msg_remote_pay_reject            = "QR reject"
		msg_no_network                   = "no network"
		pic_QR_pay_error                 = ""
		pic_pay_reject                   = ""
		light_sheduler                   = ""
	}
	service { reset_sec = 10 }
}
sound {}
watchdog {}
`

// testConfigWith добавляет дополнительный HCL к минимальному конфигу.
// extra должен содержать engine { profile {} ... } и при необходимости inventory {}.
func testConfigWith(extra string) string {
	return minimalTestConfig + extra
}
