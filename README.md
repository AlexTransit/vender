# What
Vender is free open source VMC (Vending Machine Controller). (fork https://github.com/temoto/vender)
This software has been working since 2019 (small network of coffee machines.)
equipment used:
VMC: eVend8 (bill validators: JCM300 and ICT7, Coin validators: CoinCo Global2 and Coges Aeterna)
Main Board: OrangePi Lite, 
additional display: 320x240 controller st7789v 
additional sound card: noname USB sound card


Status:
- MDB adapter hardware module - works
- VMC - works


# Hardware

Required for VMC:
- Works on RaspberryPI, OrangePi Lite (H3) OrangePi PC2(H5). Possibly anything with GPIO that runs Go/Linux.
- MDB signal current limiter - required, see files in `hardware/schematic` (https://namanow.org/wp-content/uploads/Multi-Drop-Bus-and-Internal-Communication-Protocol.pdf)
- MDB adapter, takes care of 9bit and timing, we use ATMega328p (168) with `hardware/mega-firmware`. 
    (software option is available: https://github.com/temoto/iodin . in a similar solution, another author made measurements. delay may be up to 10ms. in the specification MDB the maximum delay is 5ms )

Supported peripherals:
- MDB coin acceptor, bill validator
- Evend MDB drink devices
- any MDB device via configuration scenarios (work in progress)
- MT16S2R HD44780-like text display
- TWI(I2C) numpad keyboard
- graphic display (anyone registered in the system.)


# Design

VMC overall structure:
- engine (see internal/engine packages) executes actions, handles concurrency and errors
- device/feature drivers provide actions to engine
- configuration scenario specifies action groups and when to execute them


# Build

- Install latest Go from https://golang.org/dl/ (now worked on 1.23)
- Set target environment, default is `GOARCH=arm GOOS=linux`
- Run `script/build`
- Deploy file `build/vender` to your hardware
