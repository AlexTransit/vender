#ifndef _INCLUDE_FROM_MAIN
#error this file looks like standalone C source, but actually must be included in main.c
#endif
#ifndef INCLUDE_TWI_C
#define INCLUDE_TWI_C
#include <avr/interrupt.h>
#include <inttypes.h>
#include <stdbool.h>
#include <util/atomic.h>
#include <util/twi.h>
#include "buffer.h"
#include "common.h"
#include "crc.h"

static uint8_t const TWCR_ACK = _BV(TWINT) | _BV(TWEA) | _BV(TWEN) | _BV(TWIE);
static uint8_t const TWCR_NACK = _BV(TWINT) | _BV(TWEN) | _BV(TWIE);
static uint8_t const TWCR_STOP =
    _BV(TWSTO) | _BV(TWINT) | _BV(TWEN) | _BV(TWIE);

// Кольцевой буфер событий клавиатуры. Пишет ISR, читает main loop.
// Размер степень двойки, индексы uint8_t переполняются корректно.
#define TWI_RING_SIZE 64
#define TWI_RING_MASK (TWI_RING_SIZE - 1)
// Порция для одного ответа. Пакет: 49 байт полезной части,
// 3 байта заголовка clock, 2 байта заголовка поля, запас под ошибку.
#define TWI_CHUNK 32

static uint8_t volatile twi_ring[TWI_RING_SIZE];
static uint8_t volatile twi_head;      // пишет ISR
static uint8_t volatile twi_tail;      // пишет main loop

static void twi_init_slave(uint8_t const address) {
  TWCR = 0;
  TWBR = 0;  // TWI slave does not control clock
  TWAR = address << 1;
  TWSR = 0;
  twi_head = 0;
  twi_tail = 0;
  TWCR = TWCR_ACK;
}

// Копирует до max байт из кольца в chunk. Возвращает число скопированных.
// Вызывать внутри ATOMIC_BLOCK или из контекста, где ISR не работает.
static uint8_t twi_ring_take(uint8_t *const chunk, uint8_t const max) {
  uint8_t avail = (uint8_t)(twi_head - twi_tail);
  if (avail > max) {
    avail = max;
  }
  // события всегда парами, поэтому avail четный при avail <= max и max четном
  for (uint8_t i = 0; i < avail; i++) {
    chunk[i] = twi_ring[(uint8_t)(twi_tail + i) & TWI_RING_MASK];
  }
  twi_tail = (uint8_t)(twi_tail + avail);
  return avail;
}

static void twi_step(void) {
  if (twi_head == twi_tail) {
    return;
  }
  // MDB и ответ заняты: события остаются в кольце, ISR продолжает писать
  if (!response_empty() || mdb.state != MDB_STATE_IDLE) {
    return;
  }

  uint8_t chunk[TWI_CHUNK];
  uint8_t n;
  ATOMIC_BLOCK(ATOMIC_RESTORESTATE) {
    n = twi_ring_take(chunk, TWI_CHUNK);
  }
  response_begin(RESPONSE_TWI_LISTEN);
  response_fn(FIELD_TWI_DATA, chunk, n);
  response.filled = true;
}

// Вызывается из mdb_handle_done() внутри main loop.
// Переносит события в MDB-ответ, если в нем есть место для порции.
static inline void twi_flush_to_response(void) {
  uint8_t const avail = (uint8_t)(twi_head - twi_tail);
  if (avail == 0) {
    return;
  }
  // запас: заголовок поля 2 байта и хвост ответа
  if (response.b.length + avail + 2 + RESPONSE_RESERVED_FOR_ERROR >=
      PACKET_FIELDS_MAX_LENGTH) {
    return;
  }
  uint8_t chunk[TWI_CHUNK];
  uint8_t const n = twi_ring_take(chunk, TWI_CHUNK);
  response_fn(FIELD_TWI_DATA, chunk, n);
}

// Standard speed 100KHz = 160 clocks at F_CPU=16MHz.
// 200KHz = 80 clocks. Keep this handler short.
// Событие кладется парой (0, data), как и раньше, для совместимости формата.
ISR(TWI_vect) {
  uint8_t data;

  switch (TW_STATUS) {
    case TW_NO_INFO:
      return;

    case TW_BUS_ERROR:
      // события в кольце сохраняем, только сбрасываем шину
      TWCR = TWCR_STOP;
      return;

    // Receive SLA+R LP
    case TW_SR_ARB_LOST_SLA_ACK:
    case TW_SR_ARB_LOST_GCALL_ACK:
      TWDR = 0;
      TWCR = TWCR_ACK;
      return;

    // Receive SLA+W
    case TW_SR_SLA_ACK:
    case TW_SR_GCALL_ACK:
      TWCR = TWCR_ACK;
      return;

    // data received, ACK returned
    case TW_SR_DATA_ACK:
    case TW_SR_GCALL_DATA_ACK:
      data = TWDR;
      if ((uint8_t)(TWI_RING_SIZE - (uint8_t)(twi_head - twi_tail)) >= 2) {
        twi_ring[twi_head & TWI_RING_MASK] = 0;
        twi_ring[(uint8_t)(twi_head + 1) & TWI_RING_MASK] = data;
        twi_head = (uint8_t)(twi_head + 2);
        TWCR = TWCR_ACK;
      } else {
        TWCR = TWCR_NACK;  // буфер полон: сообщаем мастеру
      }
      return;

    // data received, NACK returned
    case TW_SR_DATA_NACK:
    case TW_SR_GCALL_DATA_NACK:
      (void)TWDR;
      TWCR = TWCR_ACK;
      return;

    // Receive SLA+R
    case TW_ST_SLA_ACK:
      TWDR = 0;
      TWCR = TWCR_ACK;
      return;

    // Send Byte Receive ACK
    case TW_ST_DATA_ACK:
      TWDR = 0;
      TWCR = TWCR_NACK;
      return;

    // Send Last Byte Receive ACK
    case TW_ST_DATA_NACK:
    // Send Last Byte Receive NACK
    case TW_ST_LAST_DATA:
    // Receive Stop or ReStart
    case TW_SR_STOP:
      TWCR = TWCR_ACK;
      return;

    default:
      TWCR = TWCR_ACK;
  }
}

#endif  // INCLUDE_TWI_C