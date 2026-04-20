# RU: UpgradeScript - скрипт, запускаемый для обновления системы после команды vmc.upgrade!.
# EN: UpgradeScript - a script run to update the system after the vmc.upgrade command!
# Example: sudo -u vmc git -C /home/vmc/vender-distr/ pull && sudo -u vmc -i /home/vmc/vender-distr/script/build && rsync -av /home/vmc/vender-distr/build/vender /home/vmc/ && logger upgrade complete. run reload triger
upgrade_script   = ""
# EN: ScriptIfBroken is executed when the system is broken
script_if_broken = ""
# EN: ErrorFolder where error logs are stored
error_folder     = "/home/vmc/vender-db/errors/"
# EN: BrokenFile for broken state logging
broken_file      = "/home/vmc/broken"

# EN: Inventory configuration for stock management
inventory {
# RU: Файл для хранения инвентаря. В нем хранится количество каждого склада в виде int32 (для возможности сохранения в CMOS). Код склада соответствует позиции в файле.
# Example: если у вас 3 склада, то в файле будет 12 байт. Первые 4 байта - количество первого склада, следующие 4 байта - количество второго склада, и т.д.
  stock_file = "/home/vmc/vender-db/inventory/store.file"

# RU: список ингридиентов. название ингридиента должно быть уникальным. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
# RU: name - название ингридиента ( иникальное значение)
  ingredient "sugar" {
# RU: min - минимальный остаток ( если остаток меньше минимума, то будет отказ в отгрузке. если минимум не указан то огружает в минус. может быть отрицательным числом)
    min = 50 
# RU: расход на единицу. какое количество ингридиента расходуется при отгрузке 1 единицы товара.
    spend_rate = 0.86 
# RU: level - уровень (плотность) ингридиента указыватеся как "x(y)" x - метка на бункере, y - вес пример: "0.5(200) 1(360) 2(680) 3.1(1020)"
    level = "1(330) 2(880)" 
# RU: tuning_key - название кнопки коррекции отгрузки. подробне в описании кнопки коррекции кнопка коррекции. имеет название и значение по умолчанию (обычно 4) можно указать максимальное значение. единица значения равна 25%. например: базовое = 4 и это 100% если указать 2 то это 50%. если 8 то это 200%
    tuning_key = "sugar"
# RU: cost - закупочная цена. нужна дял расчета себестоимости
    cost = 0.080
  }
# RU: список ингридиентов. название ингридиента должно быть уникальным. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
# RU: name - название ингридиента ( иникальное значение)
  ingredient "amaretto" {
# RU: min - минимальный остаток ( если остаток меньше минимума, то будет отказ в отгрузке. если минимум не указан то огружает в минус. может быть отрицательным числом)
    min = 50
# RU: расход на единицу. какое количество ингридиента расходуется при отгрузке 1 единицы товара.
    spend_rate = 0.42 
# RU: level - уровень (плотность) ингридиента указыватеся как "x(y)" x - метка на бункере, y - вес пример: "0.5(200) 1(360) 2(680) 3.1(1020)"
    level = "1(262) 2(700)"
# RU: cost - закупочная цена. нужна дял расчета себестоимости
    cost = 0.735
  }		
# RU: список бункеров. название склада и код одинаковые. название ингридиента. должно соответствовать названию в блоке ингридиентов. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
  stock "1" {
    code = 1
# RU: название ингридиента. должно соответствовать названию в блоке ингридиентов. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
    ingredient = "sugar"
# RU: дейсвия при отгрузке. создается команда: название_ингридиента.run(?) которая выполняет эти действия.
    register_add = "h18_position evend.hopper1.run(?) h1_shake " 	
  }
# RU: список бункеров. название склада и код одинаковые. название ингридиента. должно соответствовать названию в блоке ингридиентов. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
  stock "2" {
    code = 2
# RU: название ингридиента. должно соответствовать названию в блоке ингридиентов. нужно для связи склада и ингридиента. пока движок не переделан - это уникальное значение
    ingredient = "amaretto"
# RU: дейсвия при отгрузке. создается команда: название_ингридиента.run(?) которая выполняет эти действия.
    register_add = "h27_position evend.hopper2.run(?) h27_shake "  
  }	
}

# RU: настройки для валидаторов
# EN: Money configuration for payment settings
money {
# RU: десятичная точка.
  scale        = 100
# RU: максимальный кредит. сумма после которой аппарат откажется принимать деньги.
  credit_max   = 100
# RU: минимальная купюра. сумма, меньше которой аппарат не будет принимать деньги.
  minimal_bill = 10
# RU: максимальная купюра. сумма, больше которой аппарат не будет принимать деньги.
  maximum_bill = 200
}

# RU: Конфигурация для аппаратной части
# EN: Hardware configuration for devices
hardware {
# RU: список устройств.
# RU: имя устройства, например "mixer", "hopper1", "hopper2" и т.д. Должно быть уникальным в рамках конфигурации.
  device "example" {
# RU: required - если true, то устройство обязательно для работы системы. Если устройство с таким именем не будет найдено, система перейдет в состояние "сломано".
    required = true
# RU: disabled - если true, то устройство будет отключено. Система будет игнорировать его отсутствие и не будет пытаться с ним взаимодействовать. Это может быть полезно для устройств, которые не всегда нужны или для временного отключения устройства без удаления его конфигурации.
    disabled = false
  }

  evend {

# RU: блок вызачи стаканов.
# EN: Cup dispensing block.
    cup {
# RU: Время в секундах, через которое стакан считается не выданным и устройство может начать выдавать новый стакан.
# EN: Time in seconds after which a cup is considered not dispensed and the device can start dispensing a new cup.
      timeout_sec = 60
    }

# RU: блок управления подъемником.
# EN: Elevator control block.
    elevator {
      move_timeout_sec = 100
    }

# RU: блок управления эспрессо.
# EN: Espresso control block.
    espresso {
      timeout_sec = 300
    }

# RU: блок управления клапанами.
# EN: Valve control block.
    valve {
      temperature_hot = 86
    }
  }

# RU: графический дисплей.
# EN: graphical display.
  display {
# RU: путь к фреймбуферу для графического дисплея.
# EN: path to framebuffer for graphical display.
    framebuffer = "/dev/fb0"
  }

# RU: Конфигурация для LCD 16х2 дисплея HD44780
# EN: Configuration for LCD 16x2 HD44780 display
  hd44780 {
# RU: включить поддержку HD44780 дисплея.
    enable   = true
# RU: путь к gpiochip для управления пинами дисплея.
    pin_chip = "/dev/gpiochip0"

# RU: распиновка для четырех битного подключения дисплея. RS, RW, E, D4, D5, D6, D7. Значения должны быть номерами пинов на gpiochip.
    pinmap {
      rs = "13"
      rw = "14"
      e  = "110"
      d4 = "68"
      d5 = "71"
      d6 = "2"
      d7 = "21"
    }

# RU: ширина дисплея в символах.
    width        = 16
# RU: мигание курсора.
    blink        = false
# RU: отображение курсора.
    cursor       = false
# RU: задержка при прокрутке текста в миллисекундах.
    scroll_delay = 210
  }

  iodin_path = ""

# RU: Конфигурация для устройств ввода.
  input {

    evend_keyboard {
      enable = true
    }

    service_key = ""
  }

# RU: Конфигурация для MDB шины.
# EN: Configuration for MDB bus.
  mdb {

# RU: Конфигурация для купюроприемника.
    bill {
      scaling_factor = 0
    }

# RU: Конфигурация для монетоприемника.
    coin {
# RU: Стратегия выдачи сдачи. 0 = равномерная выдача (стараемся держать одинаковое количество монет в каждой тубе), 1 = сначала полная трубка (если туба полная то выдаем из нее. далее выдаем минимальным количеством монет), 2 = минимальное количество монет (выдаем минимальным количеством монет).
      dispense_strategy = 0
    }

    log_debug   = false
    uart_device = ""
    uart_driver = "mega"
  }

# RU: Конфигурация для устройства шлюза. одноплатник не умеет работать с 9 битным UART. для работы с MDB устройствами нужен драйвер. драйвер сделал на Atmel 168 или 328. к драйверу подключена evend клавиатура по i2c и mdb шина. общение с драйверов идет поверх SPI.
  mega {
# RU: логирование отладочной информации от драйвера Mega. может быть полезно для диагностики проблем с MDB устройствами и клавиатурой evend.
    log_debug = false
    spi       = ""
# RU: скорость SPI соединения с драйвером Mega.
    spi_speed = "100kHz"
# RU: gpiochip для прерывания от меги (когда приходит событие от MDB устройства или клавиатуры evend).
    pin_chip  = "/dev/gpiochip0"
# RU: номер пина на gpiochip для прерывания от меги.
    pin       = "6"
  }
}

# EN: Persist        PersistStruct          `hcl:"persist,block"` Tele configuration for telemetry
tele {
  enable                      = false
  vm_id                       = 0
  log_debug                   = false
  keepalive_sec               = 0
  ping_timeout_sec            = 0
  mqtt_broker                 = ""
  mqtt_log_debug              = false
  mqtt_password               = ""
  store_path                  = ""
  network_restart_timeout_sec = 0
  network_restart_script      = ""
}

# EN: UI_config for user interface settings
ui {
  log_debug = false

  front {
# RU: Сообщение об ошибке при выборе пункта меню. Например, при недоступности пункта меню (если мало ингредиентов).
    msg_menu_error                  = "ОШИБКА"
# RU: Сообщение при приготовлении напитка.
    msg_wait                        = "пожалуйста, подождите"
# RU: Сообщение при невалидной температуре воды. Например, если вода слишком холодная или слишком горячая для приготовления напитка.
    msg_water_temp                  = "температура: %d"
# RU: Сообщение если не указали код напитка.
    msg_menu_code_empty             = "Укажите код."
# RU: Сообщение при невалидном коде меню. такого кода нет.
    msg_menu_code_invalid           = "Неправильный код"
# RU: Сообщение при недостаточном кредите для выбранного напитка. первая строка.
# Example: "мало денег" или "not enough credit"
    msg_menu_insufficient_credit_l1 = "Мало денег"
# RU: Сообщение при недостаточном кредите для выбранного напитка. вторая строка.
# Example: "дали: 10 нужно: 25"
    msg_menu_insufficient_credit_l2 = "дали:%s нужно:%s"
# RU: Сообщение при недоступности выбранного напитка. мало ингредиентов или напиток отключен.
# Example: "не доступен. Выберите другой, или вернем деньги."
    msg_menu_not_available          = "Не доступен. Выберите другой, или вернем деньги."
# RU: Сообщение для опции "сливки" в меню напитков.
# Example: "сливки" или "cream"
    msg_cream                       = "Сливки"
# RU: Сообщение для опции "сахар" в меню напитков.
# Example: "сахар" или "sugar"
    msg_sugar                       = "Caxap"
# RU: Сообщение для отображения текущего кредита пользователя.
# Example: "Кредит: 25" или "Credit: 25"
    msg_credit                      = "Кредит: "
# RU: Сообщение для отображения введенного пользователем кода напитка.
# Example: "Код: 123" или "Code: 123"
    msg_input_code                  = "Код: %s"
# RU: Сообщение для отображения цены выбранного напитка.
# Example: "Цена: 25р." или "Price: 25"
    msg_price                       = "цена:%sp."
# RU: Сообщение для отображения информации о удаленной оплате, например при оплате по QR коду.
# Example: "QR " или "QR "
    msg_remote_pay                  = "QR "
# RU: Сообщение при отправке запроса на удаленную оплату, например при оплате по QR коду.
# Example: "запрос QR кода" или "QR request sent"
    msg_remote_pay_request          = "запрос QR кода"
# RU: Сообщение при отказе банка в удаленной оплате, например при оплате по QR коду.
# Example: "Банк послал :(" или "Bank refused :("
    msg_remote_pay_reject           = "Банк послал :("
# RU: Сообщение при отсутствии сети для удаленной оплаты.
# Example: "нет связи :(" или "No network :("
    msg_no_network                  = "нет связи :("
# RU: Время в секундах, через которое будет сбрасываться введенный код напитка, кредит и тюнинг сахара и сливок. отображение вернется к экрану ожидания.
    reset_sec                       = 300
# RU: Путь до картинки c ошибкой при оплате по QR коду.
# Example: "/home/vmc/pic-qrerror"
    pic_QR_pay_error                = "/home/vmc/pic-qrerror"
# RU: Путь до картинки c ошибкой при отказе оплаты по QR коду.
# Example: "/home/vmc/pic-pay-reject"
    pic_pay_reject                  = "/home/vmc/pic-pay-reject"
# RU: Расписание включения витрины. Например, "(* 06:00-23:00)" - включать подсветку каждый день с 6 утра до 11 вечера.
    light_sheduler                  = "(* 06:00-23:00)"
  }

  service {
# RU: Сценарии для тестов в сервисном меню. указывается имя теста и список действий для выполнения.
    test "boiler-fill" {
    scenario = "evend.valve.reserved_on evend.valve.pump_start sleep(2s) evend.valve.pump_stop evend.valve.reserved_off"
  }
# RU: Сценарии для тестов в сервисном меню. указывается имя теста и список действий для выполнения.
    test "conveyor" {
    scenario = " conveyor_test"
  }
# RU: Сценарии для тестов в сервисном меню. указывается имя теста и список действий для выполнения.
    test "lift" {
    scenario = "elevator_test"
  }
# RU: Сценарии для тестов в сервисном меню. указывается имя теста и список действий для выполнения.
    test "mixer" {
    scenario = "mix_poorly(10)"
  }

# RU: Время в секундах, через которое будет сделан выход из сервисного меню.
    reset_sec = 1800
  }
}

# RU: Конфигурация для звука
# EN: Sound configuration for audio settings
sound {
# RU: Если true, то звук отключен.
# EN: If true, the sound is disabled.
  disabled       = false
# RU: Громкость звука в процентах. 100 - это 100%, 50 - это 50% и т.д.
# EN: Sound volume in percent. 100 is 100%, 50 is 50%, etc.
  default_volume = 100
# RU: Директрория с аудио файлами. Файлы должны быть в формате MP3, 16 бит, 22050 Гц, моно.
# EN: Directory with audio files. Files must be in MP3 format, 16 bit, 22050 Hz, mono.
  folder         = "/home/vmc/vender-db/audio/"
# RU: файл для звука при нажатии кнопки.
# EN: File for sound when a button is pressed.
  keyBeep        = "bb.mp3"
# RU: файл для звука при внесении денег.
# EN: File for sound when money is inserted.
  moneyIn        = "moneyIn.mp3"
# RU: Команда для генерации TTS звука. текст для озвучивания.
# EN: Command for generating TTS sound. Text To Sound.
# Example: ["/home/vmc/vender-db/audio/tts/piper", "--model", "/home/vmc/vender-db/audio/tts/ruslan/voice.onnx", "--config", "/home/vmc/vender-db/audio/tts/ruslan/voice.json"]
  tts_exec       = ["/home/vmc/vender-db/audio/tts/piper", "--model", "/home/vmc/vender-db/audio/tts/ruslan/voice.onnx", "--config", "/home/vmc/vender-db/audio/tts/ruslan/voice.json"]
}

# RU: Конфигурация для системы наблюдения за сервисом. Если стророжевую собаку не кормить то сервис будет перезапущен.
watchdog {
# RU: Если true, то WatchDog для systemd отключен. Если отключить WatchDog, то при зависании сервиса он не будет перезапущен.
# EN: If true, the WatchDog for systemd is disabled. If you disable the WatchDog, then when the service hangs, it will not be restarted.
  disabled = false
# RU: когда автомат закончил готовить то создается папка на RamDrive. если в момент приготовления было отключение питания, то папки нет и нужно сделать инициальзацию с выдачей стакана.
# EN: when the automaton finished preparing, a folder is created on RamDrive. if there was a power outage during preparation, then there is no folder and initialization with cup dispensing is required.
  folder   = "/run/vender/"
}

# RU: Конфигурация для движка. В ней описано как готовить напитки, какие вложенные сценарии использовать и т.д.
engine {
# RU: список псевдонимов.
# RU: имя псевдонима.
  alias "example" {
# RU: сценарий, который будет выполнен при вызове псевдонима.
    scenario = "example_scenario"
# RU: сценарии действий если произошла ошибка. Ключ - регулярное выражение кода ошибки. на каждую ошибку свой сценарий. не допускается пересечение регулярных выражений для разных ошибок.
# EN: error scenarios. Key - regular expression of the error code. each error has its own scenario. overlapping of regular expressions for different errors is not allowed.
# Example: onError "3[78]" { scenario = "error_scenario" } - при ошибках с кодами 37,38 будет выполняться сценарий "error_scenario"
    onError "3[78]" { // this will match error codes 37 and 38
      scenario = "error_scenario"
    }
  }
# RU: список псевдонимов.
# RU: имя псевдонима.
  alias "example1" {
# RU: сценарий, который будет выполнен при вызове псевдонима.
    scenario = "example_scenario1"
# RU: сценарии действий если произошла ошибка. Ключ - регулярное выражение кода ошибки. на каждую ошибку свой сценарий. не допускается пересечение регулярных выражений для разных ошибок.
# EN: error scenarios. Key - regular expression of the error code. each error has its own scenario. overlapping of regular expressions for different errors is not allowed.
# Example: onError "3[78]" { scenario = "error_scenario" } - при ошибках с кодами 37,38 будет выполняться сценарий "error_scenario"
    onError "3" {
      scenario = "error_scenario1"
    }
# RU: сценарии действий если произошла ошибка. Ключ - регулярное выражение кода ошибки. на каждую ошибку свой сценарий. не допускается пересечение регулярных выражений для разных ошибок.
# EN: error scenarios. Key - regular expression of the error code. each error has its own scenario. overlapping of regular expressions for different errors is not allowed.
# Example: onError "3[78]" { scenario = "error_scenario" } - при ошибках с кодами 37,38 будет выполняться сценарий "error_scenario"
    onError "4" {
      scenario = "error_scenario2"
    }
# RU: сценарии действий если произошла ошибка. Ключ - регулярное выражение кода ошибки. на каждую ошибку свой сценарий. не допускается пересечение регулярных выражений для разных ошибок.
# EN: error scenarios. Key - regular expression of the error code. each error has its own scenario. overlapping of regular expressions for different errors is not allowed.
# Example: onError "3[78]" { scenario = "error_scenario" } - при ошибках с кодами 37,38 будет выполняться сценарий "error_scenario"
    onError "\d{2}" { // this will match any 2 digit error code, but if there are more specific regexes for 30-39, they will take precedence over this one
      scenario = "error_scenario2"
    }
  }
# RU: список меню.
  menu {
# RU: код напитка. должен быть уникальным для каждого напитка.
    item "43." {
# RU: имя напитка.
      name = "горячий шоколад со сливками и орешками" 
# RU: цена напитка в копейках. например, 25 рублей это 2500 копеек. если цена 0, то напиток будет бесплатным.
      price = 80 
# RU: максимальное количество сливок для этого напитка. если 0, то будет использоваться значение из DefaultCreamMax.
      creamMax = 4 
# RU: максимальное количество сахара для этого напитка. если 0, то будет использоваться значение из DefaultSugarMax.
      sugarMax = 4 
# RU: сценарий приготовления напитка. может содержать псевдонимы.
      scenario = " preset add.sugar(5) add.chocolate(40) cream20 mix_strong w_hot70 cup_serve_p "
    }
# RU: код напитка. должен быть уникальным для каждого напитка.
    item "4" {
# RU: отключить напиток. если true, то напиток не будет отображаться в меню и не будет доступен для приготовления.
      disabled = true
    }
# RU: код напитка. должен быть уникальным для каждого напитка.
    item "31" {
# RU: имя напитка.
      name = "кофе с шоколадом и орешками" 
# RU: цена напитка в копейках. например, 25 рублей это 2500 копеек. если цена 0, то напиток будет бесплатным.
      price = 60 
# RU: сценарий приготовления напитка. может содержать псевдонимы.
      scenario = " preset add.coffee(5) add.chocolate(30) mix_midle w_hot85 add.peanut(7) cup_serve_p "
    }
  }
# RU: список действий при загрузке системы.
# Example: on_boot = ["text_boot sleep(2s)", "evend.cup.ensure", "evend.valve.set_temp_hot_config " ]
  on_boot          = null
# RU: список действий при первоначальной загрузке системы. проверяет папку Watchdog. если папки нет, то выполняет эти действия. после выполнения создает папку. если папка есть, то эти действия не выполняются.
# Example: first_init = ["text_first_init evend.cup.ensure", "evend.valve.set_temp_hot_config " ]
  first_init       = null
# EN: Deprecated:
  on_menu_error    = null
# RU: список действий при входе в сервисное меню.
# Example: on_service_begin = [ " evend.valve.set_temp_hot(0) ", " evend.cup.light_off " ]
  on_service_begin = null
# RU: список действий при выходе из сервисного меню.
# Example: on_service_end = [ " evend.valve.set_temp_hot_config " ]
  on_service_end   = null
# RU: список действий при начале ожидания действий от клиента. выполняется поле окончания приготовления и после timeout UI.
# Example: on_front_begin = ["text_reklama picture(/home/vmc/pic-coffe) evend.valve.set_temp_hot_config evend.cup.light_on_schedule ", ]
  on_front_begin   = null
# RU: список действий если произошла ошибка.
# Example: on_broken = ["text_broken picture(/home/vmc/pic-broken) picture(/home/vmc/pic-broken) money.abort evend.cup.light_off play(broken.mp3) " ]
  on_broken        = null
# RU: список при выходе из программы.
# Example: on_shutdown = [ "text_poweroff picture(/home/vmc/pic-broken) money.abort evend.cup.light_off evend.valve.set_temp_hot(0) " ]
  on_shutdown      = null

  profile {
    regexp     = ""
    min_us     = 0
    log_format = ""
  }
}
