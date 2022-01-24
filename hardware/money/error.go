package money

import "fmt"

var (
	ErrSensor      = fmt.Errorf("Defective_Sensor")
	ErrNoStorage   = fmt.Errorf("Storage_Unplugged")
	ErrJam         = fmt.Errorf("Jam")
	ErrROMChecksum = fmt.Errorf("ROM_checksum")
	ErrFraud       = fmt.Errorf("Possible_Credited_Money_Removal")
	ErrFishingOK   = fmt.Errorf("ALERT_!!!!_Credited_Money_Removal._good_fishing")
	ErrFishingFail = fmt.Errorf("maybe_alert_!!!!_Bill_money_fishing_fail")
	ErrBillReject  = fmt.Errorf("Bill_rejected")
)
