upgrade_script   = "sudo -u vmc git -C /home/vmc/vender-distr/ pull && sudo -u vmc -i /home/vmc/vender-distr/script/build && rsync -av /home/vmc/vender-distr/build/vender /home/vmc/ && logger upgrade complete. run reload triger"
script_if_broken = "journalctl -e -n100 > /home/vmc/vender-db/errors/br-`date '+%Y-%m-%d_%H_%M_%S'`"
error_folder     = ""

inventory {
  stock_file = "/home/vmc/vender-db/inventory/store.file"

  ingredient "sugar" {
    min        = 50
    spend_rate = 0.82
    level      = "1(330) 2(880)"
    tuning_key = "sugar"
  }
  ingredient "peanut" {
    min        = 60
    spend_rate = 0.3
    level      = "0.5(200) 1(360) 2(680) 3.1(1020)"
  }
  ingredient "water" {
    level = "1(1000)"
    spend_rate = 0.72
  }
  ingredient "cup"{
    level = "1(60)"
  }

  stock "1" {
    code         = 1
    ingredient   = "sugar"
    register_add = "h18_position evend.hopper1.run(?) h1_shake"
  }
  stock "8" {
    code         = 8
    ingredient   = "peanut"
    register_add = "h18_position evend.hopper8.run(?) sleep(1s)"
  }
  stock "9" {
    code = 9
    ingredient = "water"
  }
    stock "10" {
    ingredient = "cup"
  }
}

money {
  scale      = 100
  credit_max = 10000
}

hardware {
  device "bill" { }
  device "coin" { required = true }
  device "evend.cup" { required = true }
  device "evend.valve" { required = true }
  device "evend.mixer" { required = true }
  device "evend.hopper1" { }
  device "evend.hopper2" { disabled = true }

  evend {

    cup {
      timeout_sec = 60
    }

    elevator {
      move_timeout_sec = 100
    }

    espresso {
      timeout_sec = 300
    }

    valve {
      temperature_hot = 86
    }
  }

  display {
    framebuffer = "/dev/fb0"
  }

  hd44780 {
    enable   = true
    codepage = "windows-1251"
    pin_chip = "/dev/gpiochip1"

    pinmap {
      rs = "69"
      rw = "70"
      e  = "110"
      d4 = "68"
      d5 = "71"
      d6 = "2"
      d7 = "21"
    }

    page1        = true
    width        = 16
    blink        = false
    cursor       = false
    scroll_delay = 210
  }

  iodin_path = ""

  input {

    evend_keyboard {
      enable = true
    }

    service_key = ""
  }

  mdb {

    bill {
      scaling_factor = 0
    }

    coin {
      dispense_strategy = 0
    }

    log_debug   = false
    uart_device = ""
    uart_driver = "mega"
  }

  mega {
    log_debug = false
    spi       = ""
    spi_speed = "100kHz"
    pin_chip  = "/dev/gpiochip1"
    pin       = "6"
  }
}

tele {
  enable                      = true
  vm_id                       = 88
  log_debug                   = false
  keepalive_sec               = 0
  ping_timeout_sec            = 0
  mqtt_broker                 = "tcp://domain.name:port"
  mqtt_log_debug              = false
  mqtt_password               = ""
  store_path                  = ""
  network_restart_timeout_sec = 10
  network_restart_script      = ""
}

ui {
  log_debug = false

  front {
    msg_menu_error                  = "ERROR"
    msg_wait                        = "Wait, please"
    msg_water_temp                  = "temperature: %d"
    msg_menu_code_empty             = "Code empty" // "Закончился ингридиент. Выверите другой ."
    msg_menu_code_invalid           = "Code invalid" // "Неправильный код"
    msg_menu_insufficient_credit_l1 = "Money Low" // "Мало денег" 
    msg_menu_insufficient_credit_l2 = "dali:%s nuno:%s"
    msg_menu_not_available          = "Ne dostupno "
    msg_cream                       = "Cream" // "Сливки"
    msg_sugar                       = "Sugar" // "Caxap"
    msg_credit                      = "Credit" //  "Кредит: "
    msg_input_code                  = "Code: %s"
    msg_price                       = "price:%sp." // "цена:%sp."
    msg_remote_pay                  = "QR "
    msg_remote_pay_request          = "zapros QR"
    msg_remote_pay_reject           = "bank say - get lost! :(" // "Банк послал :("
    msg_no_network                  = "HET internet :("
    reset_sec                       = 300
    pic_QR_pay_error                = "/home/vmc/pic-qrerror"
    pic_pay_reject                  = "/home/vmc/pic-pay-reject"
    light_sheduler                  = "(* 06:00-23:00)"
  }

  service {
    reset_sec = 1800
    test "conveyor" { scenario=" conveyor_test"}
    test "lift" { scenario="elevator_test" }
    test "mixer" { scenario="mix_poorly(10)" }
  }
}

sound {
  disabled       = false
  default_volume = 10
  folder         = "/home/vmc/vender-db/audio/"
  keyBeep        = "bb.mp3"
  moneyIn        = "moneyIn.mp3"
}

watchdog {
  disabled = false
  folder   = "/run/user/1000/"
}

engine {
  on_boot          = ["text_boot sleep(2s)", "evend.cup.ensure", "evend.valve.set_temp_hot_config "]
  first_init       = ["text_test picture(/home/vmc/pic-broken) sound(start.mp3) init_devices "]
  on_menu_error    = [" money.abort "]
  on_service_begin = [" evend.valve.set_temp_hot(0) ", " evend.cup.light_off "]
  on_service_end   = [" evend.valve.set_temp_hot_config "]
  on_front_begin   = ["text_reklama picture(/home/vmc/pic-coffe) evend.valve.set_temp_hot_config evend.cup.light_on_schedule "]
  on_broken        = ["text_broken picture(/home/vmc/pic-broken) sound(broken.mp3) picture(/home/vmc/pic-broken) money.return evend.cup.light_off "]
  on_shutdown      = ["text_poweroff picture(/home/vmc/pic-broken) money.abort evend.cup.light_off evend.valve.set_temp_hot(0) "]

  profile {
    regexp     = ""
    min_us     = 500
    log_format = "-------------------- action=%s time=%s"
  }
  
  alias "h18_position" { scenario = "evend.conveyor.move(1210)" }

  menu {
    default_cream     = 4
    default_cream_max = 6
    default_sugar     = 4
    default_sugar_max = 8

    item "1" {
      name = "кофе со сливками и сахаром"
      price = 20
      scenario = "preset add.sugar(12) add.coffee(7) cream30 mix_poor w_hot95 cup_serve "
    }
    item "2" {
      name = "bla-bla"
      price = 25
      creamMax = 4
      sugarMax = 4
      scenario = "step1 step2 stepN"
    }
    item "3" { disabled = true }

  }
}

include "/home/vmc/local.hcl" { optional = true }