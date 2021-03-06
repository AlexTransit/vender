engine {
  // alias "cup_dispense" { scenario = "conveyor_move_cup cup_drop" }

  // alias "conveyor_hopper18" { scenario = "evend.conveyor.move(1210)" }

  inventory {
    persist = true

    // Send stock name to telemetry; false to save network usage
    tele_add_name = true

    // Stock fields:
    // - name string, must be non-empty and unique
    // - code uint32, default=0, sorting index in service menu, duplicates produce warning at boot but allowed
    // - check bool, default=false, validate stock remainder > `min`
    // - min float, only makes sense together with check
    // - hw_rate float, default=1, engine `add.{name}(x)` sends x*hw_rate to hardware device
    // - spend_rate float, default=1, engine `stock.{name}.spend(x)` (implied by add) subtracts x*spend_rate from remainder
    // - register_add string, registers `add.{name}(?)` in engine with this scenario, must contain `foo(?)` arg placeholder
    // stock "water" { hw_rate = 0.649999805 }
    // stock "cup" { code = 1 }

    // stock "milk" { code = 1 check = true min = 100 register_add = "conveyor_hopper18 evend.hopper1.run(?)" spend_rate = 9.7 }
  }

  menu {
    item "1" {
      name     = "example1"
      price    = 5
      scenario = "cup_drop water_hot(150) cup_serve"
    }

    item "2" {
      name     = "example2"
      price    = 1
      scenario = "cup_drop add.water_hot(10) add.milk(10) cup_serve"
    }
  }

  // first_init = ["release_cup"]
  // on_boot = ["mixer_move_top", "cup_serve", "conveyor_move_cup"]
  // on_broken = ["money.abort evend.cup.light_off evend.valve.set_temp_hot(0)"]
  // on_front_begin = []
  // on_menu_error = ["money.abort", "cup_serve"]
  // on_service_begin = []
  
  profile {
    // additional escape of \ is required
    regexp     = "^(cup_|money\\.)"
    min_us     = 500
    log_format = "engine profile action=%s time=%s"
  }
}

hardware {
  // All devices must be listed here to use.

  device "bill" {
    // If any required devices are offline, switch to broken state.
    // required=false will still probe and report errors to telemetry.
    required = true
  }

  device "coin" {
    required = true
  }

  // device "evend.cup" { required = true }
  // device "evend.hopper5" { }

  evend {
    conveyor {
      keepalive_ms = 0
      min_speed    = 200
      position_max = 0
    }

    cup {
      assert_busy_delay_ms = 30
      dispense_timeout_sec = 10
      ensure_timeout_sec   = 70
    }

    elevator {
      keepalive_ms = 0
      timeout_sec  = 10
    }

    espresso {
      timeout_sec = 30
    }

    hopper {
      run_timeout_ms = 0
    }

    mixer {
      keepalive_ms     = 0
      move_timeout_sec = 10
      shake_timeout_ms = 300
    }

    valve {
      temperature_hot      = 0
      temperature_valid_ms = 30
      pour_timeout_sec     = 600
      caution_part_ml      = 0
    }
  }
  hd44780 {
    codepage = "windows-1251"
    enable   = true
    page1    = true

    pinmap {
      rs = "23"
      rw = "18"
      e  = "24"
      d4 = "22"
      d5 = "21"
      d6 = "17"
      d7 = "7"
    }

    blink        = true
    cursor       = false
    scroll_delay = 210
    width        = 16
  }
  input {
    evend_keyboard {
      enable = true

      // TODO listen_addr = 0x78
    }

    dev_input_event {
      enable = true
      device = "/dev/input/event0"
    }
  }
  iodin_path = "TODO_EDIT"
  mega {
    spi       = ""
    spi_speed = "200kHz"
    pin_chip  = "/dev/gpiochip0"
    pin       = "25"
  }
  mdb {
    bill {
      scaling_factor = 0
    }

    coin {
      give_smart           = false
      dispense_timeout_sec = 0
    }

    // log_debug = true
    log_debug = false

    uart_driver = "mega"

    #uart_driver = "file"
    #uart_device = "/dev/ttyAMA0"

    #uart_driver = "iodin"
    #uart_device = "\x0f\x0e"
  }
}

money {
  // Multiple of lowest money unit for config convenience and formatting.
  // All money numbers in config are multipled by scale.
  // For USD/EUR set `scale=1` and specify prices in cents.
  scale = 100

  credit_max = 200
  
  // limit to over-compensate change return when exact amount is not available
  change_over_compensate = 10
}

persist {
  // database folder
  root = "./"
}

tele {
  enable              = false
  vm_id               = -1
  log_debug           = true
 	keepalive_sec       = 60 //default
	ping_timeout_sec    = 30 //default
  mqtt_broker         = "tls://TODO_EDIT:8884"
  mqtt_log_debug      = false
  mqtt_password       = "TODO_EDIT"
  store_path          = "TODO_EDIT store unsend messages"  //default '/home/vmc/vender-db/telemessages'

  network_restart_script = "TODO_EDIT script to run in case of network failure / ???????????? ?????????????????????? ?? ???????????? ???????????????????? ???????? " 
  network_restart_timeout_sec = 600 //default ( timeout before running the script / ?????????? ???????????????? ?????????? ???????????????? ?????????????? )

}

ui {
  front {
    msg_intro     = "TODO_EDIT showed after successful boot"
    msg_broken_l1 = "TODO_EDIT showed after critical error line2 (recomended <17 symbol)"
    msg_broken_l1 = "TODO_EDIT showed after critical error line2 (recomended <17 symbol)" 
    msg_locked    = "remotely locked"
    msg_wait      = "please wait"

    msg_menu_code_empty          = "Code empty" // "???????????????????? ????????????????????. ???????????????? ???????????? ."
    msg_menu_code_invalid        = "Code invalid" // "???????????????????????? ??????"
    msg_menu_insufficient_credit = "Insufficient credit" // "???????????????????????? ??????????"
    msg_menu_not_available       = "Not available" // "???? ????????????????"
    msg_cream                    = "Cream" // "????????????"
    msg_sugar                    = "Sugar" // "Caxap"
    msg_credit                   = "Credit" //  "????????????: "
    msg_making1                  = "Making text line1" // "??????????????"
    msg_making2                  = "Making text line2" // "??????????????"
    msg_input_code               = "Code:%s\x00" // "??????: %s\x00"
    msg_remote_pay_l1            = "invoice :%s\x00" // "?? ????????????: %s\x00"
    msg_remote_pay_l2            = "QR payment" // "???????????? ???? QR"

    pic_boot         = "/path/boot-picture"
    pic_idle         = "/path/idle-picture"
    pic_client       = "/path/instruction-picture"
    pic_make         = "/path/make-picture"
    pic_broken       = "/path/broken-picture"
    pic_QR_pay_error = "/path/QR-error-picture"
    pic_pay_reject   = "/path/bank-pay-reject"

    reset_sec = 180
  }
}

display { framebuffer = "/dev/fb0" }

include "local.hcl" {
  optional = true
}
