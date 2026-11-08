#ifndef INCLUDE_CONFIG_H
#define INCLUDE_CONFIG_H

#define F_CPU 16000000UL  // Clock Speed

#define FIRMWARE_VERSION 0x0204
#define MDB_TIMEOUT_MS 6
#define BUFFER_SIZE 50

#define MASTER_NOTIFY_DDR DDRD
#define MASTER_NOTIFY_PORT PORTD
#define MASTER_NOTIFY_PIN PIND2

#define SPI_MISO_DDR DDRB
#define SPI_MISO_PIN PINB4
#define SPI_SS_DDR DDRB
#define SPI_SS_PORT PINB
#define SPI_SS_PIN PINB2

// B5 is taken by SPI SCK now
// #define LED_DDR DDRB
// #define LED_PORT PORTB
// #define LED_PIN PINB5

// Debug pin: PC1 (ATmega168 DIP-28 pin 24). Toggles on every SPDR write in SPI ISR.
// Comment out DEBUG_PIN_ENABLED for production build.
#define DEBUG_PIN_ENABLED
#ifdef DEBUG_PIN_ENABLED
#define DBG_DDR DDRC
#define DBG_PORT PORTC
#define DBG_PIN PC1
#define DBG_TOGGLE() (PINC = _BV(DBG_PIN))
#endif

#endif  // INCLUDE_CONFIG_H