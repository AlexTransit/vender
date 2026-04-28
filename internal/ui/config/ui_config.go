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
	// RU: Сообщение об ошибке при выборе пункта меню. Например, при недоступности пункта меню (если мало ингредиентов).
	MsgMenuError string `hcl:"msg_menu_error"`
	// RU: Сообщение при приготовлении напитка.
	MsgWait string `hcl:"msg_wait"`
	// RU: Сообщение при невалидной температуре воды. Например, если вода слишком холодная или слишком горячая для приготовления напитка.
	MsgWaterTemp string `hcl:"msg_water_temp"`
	// RU: Сообщение если не указали код напитка.
	MsgMenuCodeEmpty string `hcl:"msg_menu_code_empty"`
	// RU: Сообщение при невалидном коде меню. такого кода нет.
	MsgMenuCodeInvalid string `hcl:"msg_menu_code_invalid"`
	// RU: Сообщение при недостаточном кредите для выбранного напитка. первая строка.
	// Example: "мало денег" или "not enough credit"
	MsgMenuInsufficientCreditL1 string `hcl:"msg_menu_insufficient_credit_l1"`
	// RU: Сообщение при недостаточном кредите для выбранного напитка. вторая строка.
	// Example: "дали: 10 нужно: 25"
	MsgMenuInsufficientCreditL2 string `hcl:"msg_menu_insufficient_credit_l2"` // дали:%s нужно:%s
	// RU: Сообщение при недоступности выбранного напитка. мало ингредиентов или напиток отключен.
	// Example: "не доступен. Выберите другой, или вернем деньги."
	MsgMenuNotAvailable string `hcl:"msg_menu_not_available"` //"Not available" // "Не доступен"
	// RU: Сообщение для опции "сливки" в меню напитков.
	// Example: "сливки" или "cream"
	MsgCream string `hcl:"msg_cream"`
	// RU: Сообщение для опции "сахар" в меню напитков.
	// Example: "сахар" или "sugar"
	MsgSugar string `hcl:"msg_sugar"`
	// RU: Сообщение для отображения текущего кредита пользователя.
	// Example: "Кредит: 25" или "Credit: 25"
	MsgCredit string `hcl:"msg_credit"`
	// RU: Сообщение для отображения введенного пользователем кода напитка.
	// Example: "Код: 123" или "Code: 123"
	MsgInputCode string `hcl:"msg_input_code"`
	// RU: Сообщение для отображения цены выбранного напитка.
	// Example: "Цена: 25р." или "Price: 25"
	MsgPrice string `hcl:"msg_price"`
	// RU: Сообщение для отображения информации о удаленной оплате, например при оплате по QR коду.
	// Example: "QR " или "QR "
	MsgRemotePay string `hcl:"msg_remote_pay"`
	// RU: Сообщение при отправке запроса на удаленную оплату, например при оплате по QR коду.
	// Example: "запрос QR кода" или "QR request sent"
	MsgRemotePayRequest string `hcl:"msg_remote_pay_request"` // "QR request sent" // "QR запрос отправлен"
	// RU: Сообщение при отказе банка в удаленной оплате, например при оплате по QR коду.
	// Example: "Банк послал :(" или "Bank refused :("
	MsgRemotePayReject string `hcl:"msg_remote_pay_reject"` // "Bank refused :(" // "Банк отказал :("
	// RU: Сообщение при отсутствии сети для удаленной оплаты.
	// Example: "нет связи :(" или "No network :("
	MsgNoNetwork string `hcl:"msg_no_network"`
	// RU: Время в секундах, через которое будет сбрасываться введенный код напитка, кредит и тюнинг сахара и сливок. отображение вернется к экрану ожидания.
	ResetTimeoutSec int `hcl:"reset_sec"`
	// RU: Путь до картинки c ошибкой при оплате по QR коду.
	// Example: "/home/vmc/pic-qrerror"
	PicQRPayError string `hcl:"pic_QR_pay_error"`
	// RU: Путь до картинки c ошибкой при отказе оплаты по QR коду.
	// Example: "/home/vmc/pic-pay-reject"
	PicPayReject string `hcl:"pic_pay_reject"`
	// RU: Расписание включения витрины. Например, "(* 06:00-23:00)" - включать подсветку каждый день с 6 утра до 11 вечера.
	LightShedule string `hcl:"light_sheduler"`
}

type ServiceStruct struct {
	// RU: Время в секундах, через которое будет сделан выход из сервисного меню.
	ResetTimeoutSec int `hcl:"reset_sec"`
	// RU: Сценарии для тестов в сервисном меню. указывается имя теста и список действий для выполнения.
	XXX_Tests []TestsStruct `hcl:"test,block"`
	Tests     map[string]TestsStruct
}

type TestsStruct struct {
	Name     string `hcl:"name,label"`
	Scenario string `hcl:"scenario"`
}

type UIUser struct {
	QrText string
	menu_config.UIMenuStruct
	UiState               uint32
	DirtyMoney            currency.Amount
	Lock                  bool
	KeyboardReadEnable    bool
	RemoteOrderInProgress bool
}
