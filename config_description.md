## upgrade_script

Type: string

Description (RU): UpgradeScript - скрипт, запускаемый для обновления системы после команды vmc.upgrade!.

Description (EN): UpgradeScript - a script run to update the system after the vmc.upgrade command!

Example: sudo -u vmc git -C /home/vmc/vender-distr/ pull && sudo -u vmc -i /home/vmc/vender-distr/script/build && rsync -av /home/vmc/vender-distr/build/vender /home/vmc/ && logger upgrade complete. run reload triger

## script_if_broken

Type: string

Description (EN): ScriptIfBroken is executed when the system is broken

## error_folder

Type: string

Description (EN): ErrorFolder where error logs are stored

## broken_file

Type: string

Description (EN): BrokenFile for broken state logging

## inventory

Type: inventory.Inventory

Description (EN): Inventory configuration for stock management

## inventory.stock_file

Type: string

Description (RU): Файл для хранения инвентаря. В нем хранится количество каждого склада в виде int32 (для возможности сохранения в CMOS). Код склада соответствует позиции в файле.

Example: если у вас 3 склада, то в файле будет 12 байт. Первые 4 байта - количество первого склада, следующие 4 байта - количество второго склада, и т.д.

## inventory.stock

Type: []inventory.Stock

Description (RU): блок складов.

## inventory.ingredient

Type: []inventory.Ingredient

## money

Type: config_global.MoneyStruct

Description (EN): Money configuration for payment settings

## money.scale

Type: int

## money.credit_max

Type: int

## money.minimal_bill

Type: int

## money.maximum_bill

Type: int

## hardware

Type: config_global.HardwareStruct

Description (EN): Hardware configuration for devices

## hardware.device

Type: []config_global.DeviceConfig

## hardware.evend

Type: evend_config.Config

## hardware.evend.cup

Type: evend_config.CupStruct

## hardware.evend.cup.timeout_sec

Type: int

## hardware.evend.elevator

Type: evend_config.ElevatorStruct

## hardware.evend.elevator.move_timeout_sec

Type: int

## hardware.evend.espresso

Type: evend_config.EspressoStruct

## hardware.evend.espresso.timeout_sec

Type: int

## hardware.evend.valve

Type: evend_config.ValveStruct

## hardware.evend.valve.temperature_hot

Type: int

## hardware.display

Type: config_global.DisplayStruct

## hardware.display.framebuffer

Type: string

## hardware.hd44780

Type: config_global.HD44780Struct

## hardware.hd44780.enable

Type: bool

## hardware.hd44780.pin_chip

Type: string

## hardware.hd44780.pinmap

Type: hd44780.PinMap

## hardware.hd44780.pinmap.rs

Type: string

## hardware.hd44780.pinmap.rw

Type: string

## hardware.hd44780.pinmap.e

Type: string

## hardware.hd44780.pinmap.d4

Type: string

## hardware.hd44780.pinmap.d5

Type: string

## hardware.hd44780.pinmap.d6

Type: string

## hardware.hd44780.pinmap.d7

Type: string

## hardware.hd44780.width

Type: int

## hardware.hd44780.blink

Type: bool

## hardware.hd44780.cursor

Type: bool

## hardware.hd44780.scroll_delay

Type: int

## hardware.iodin_path

Type: string

## hardware.input

Type: config_global.InputStruct

## hardware.input.evend_keyboard

Type: config_global.EvendKeyboardStruct

## hardware.input.evend_keyboard.enable

Type: bool

## hardware.input.service_key

Type: string

## hardware.mdb

Type: mdb_config.Config

## hardware.mdb.bill

Type: mdb_config.BillStruct

## hardware.mdb.bill.scaling_factor

Type: int

## hardware.mdb.coin

Type: mdb_config.CoinStruct

## hardware.mdb.coin.dispense_strategy

Type: int

## hardware.mdb.log_debug

Type: bool

## hardware.mdb.uart_device

Type: string

## hardware.mdb.uart_driver

Type: string

## hardware.mega

Type: config_global.MegaStruct

## hardware.mega.log_debug

Type: bool

## hardware.mega.spi

Type: string

## hardware.mega.spi_speed

Type: string

## hardware.mega.pin_chip

Type: string

## hardware.mega.pin

Type: string

## tele

Type: tele_config.Config

Description (EN): Persist        PersistStruct          `hcl:"persist,block"` Tele configuration for telemetry

## tele.enable

Type: bool

## tele.vm_id

Type: int

## tele.log_debug

Type: bool

## tele.keepalive_sec

Type: int

## tele.ping_timeout_sec

Type: int

## tele.mqtt_broker

Type: string

## tele.mqtt_log_debug

Type: bool

## tele.mqtt_password

Type: string

## tele.store_path

Type: string

## tele.network_restart_timeout_sec

Type: int

## tele.network_restart_script

Type: string

## ui

Type: ui_config.Config

Description (EN): UI_config for user interface settings

## ui.log_debug

Type: bool

## ui.front

Type: ui_config.FrontStruct

## ui.front.msg_menu_error

Type: string

## ui.front.msg_wait

Type: string

## ui.front.msg_water_temp

Type: string

## ui.front.msg_menu_code_empty

Type: string

## ui.front.msg_menu_code_invalid

Type: string

## ui.front.msg_menu_insufficient_credit_l1

Type: string

## ui.front.msg_menu_insufficient_credit_l2

Type: string

## ui.front.msg_menu_not_available

Type: string

## ui.front.msg_cream

Type: string

## ui.front.msg_sugar

Type: string

## ui.front.msg_credit

Type: string

## ui.front.msg_input_code

Type: string

## ui.front.msg_price

Type: string

## ui.front.msg_remote_pay

Type: string

## ui.front.msg_remote_pay_request

Type: string

## ui.front.msg_remote_pay_reject

Type: string

## ui.front.msg_no_network

Type: string

## ui.front.reset_sec

Type: int

## ui.front.pic_QR_pay_error

Type: string

## ui.front.pic_pay_reject

Type: string

## ui.front.light_sheduler

Type: string

## ui.service

Type: ui_config.ServiceStruct

## ui.service.reset_sec

Type: int

## ui.service.test

Type: []ui_config.TestsStruct

## sound

Type: sound_config.Config

Description (EN): Sound configuration for audio

## sound.disabled

Type: bool

## sound.default_volume

Type: int16

## sound.folder

Type: string

## sound.keyBeep

Type: string

## sound.moneyIn

Type: string

## sound.tts_exec

Type: []string

## watchdog

Type: watchdog_config.Config

## watchdog.disabled

Type: bool

## watchdog.folder

Type: string

## engine

Type: engine_config.Config

## engine.alias

Type: []engine_config.Alias

## engine.on_boot

Type: []string

## engine.first_init

Type: []string

## engine.on_menu_error

Type: []string

Description (EN): Deprecated:

## engine.on_service_begin

Type: []string

## engine.on_service_end

Type: []string

## engine.on_front_begin

Type: []string

## engine.on_broken

Type: []string

## engine.on_shutdown

Type: []string

## engine.profile

Type: engine_config.ProfileStruct

## engine.profile.regexp

Type: string

## engine.profile.min_us

Type: int

## engine.profile.log_format

Type: string

## engine.menu

Type: menu_config.XXX_MenuStruct

## engine.menu.item

Type: []menu_config.MenuItem

