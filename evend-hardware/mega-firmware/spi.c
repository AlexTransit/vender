#ifndef _INCLUDE_FROM_MAIN
#error this file looks like standalone C source, but actually must be included in main.c
#endif
#ifndef INCLUDE_SPI_C
#define INCLUDE_SPI_C
#include "config.h"

#include <avr/interrupt.h>
#include <avr/io.h>
#include <inttypes.h>
#include <stdbool.h>
#include <string.h>
#include "bit.h"
#include "buffer.h"
#include "common.h"
#include "crc.h"

static uint8_t volatile spi_last_out_crc;
static uint8_t rx_data[BUFFER_SIZE + 1];

// Rule: load the next byte right after the previous one is read.
// Any other work goes between SPI_LOAD() and the next SPI_WAIT_OR_RETURN,
// while a byte is being shifted.
#define SPI_WAIT_OR_RETURN \
  if (!spi_wait()) {       \
    return;                \
  }

#define SPI_LOAD(a) \
  SPDR = (a);       \
  DBG_TOGGLE();

// wait for end of current byte, discard received value, load next byte
#define SPI_XCHG_DISCARD(next) \
  SPI_WAIT_OR_RETURN;          \
  (void)SPDR;                  \
  SPI_LOAD(next);

// fwd

static bool spi_selected(void) __attribute__((warn_unused_result));
static bool spi_wait(void) __attribute__((warn_unused_result));
static inline void spi_end(uint8_t const errcode, uint8_t const pad);
static inline void spi_reply_empty(uint8_t const errcode, uint8_t const pad);
static inline void spi_do_send(uint8_t const payload_length,
                               uint8_t const kind);
static inline void spi_do_recv(uint8_t const in_length);
static inline void spi_do_ack(uint8_t const local_length);

static void spi_init_slave(void) {
  bit_mask_set(SPI_MISO_DDR, _BV(SPI_MISO_PIN));
  SPCR = _BV(SPE);
  SPSR = 0;

  // drop SPIF
  (void)SPSR;
  (void)SPDR;

  bit_mask_set(PCICR, _BV(PCIE0));
  bit_mask_set(PCMSK0, _BV(PCINT2));
}

static void spi_step(void) {
  // пин поднимаем, если есть готовый ответ или события в кольце TWI
  bool const pending = !response_empty() || (twi_head != twi_tail && mdb.state == MDB_STATE_IDLE);
  master_notify_set(pending && !spi_selected());
}

// Ends transaction with errcode/pad. Caller has just read a byte.
static inline void spi_end(uint8_t const errcode, uint8_t const pad) {
  SPI_LOAD(errcode);
  for (;;) {
    SPI_XCHG_DISCARD(pad);
  }
}

// Byte 3 (payload crc = 0) is in flight. Reply with errcode.
static inline void spi_reply_empty(uint8_t const errcode, uint8_t const pad) {
  SPI_WAIT_OR_RETURN;
  (void)SPDR;
  spi_end(errcode, pad);
}

// Byte 3 (kind) is in flight. payload_length was already sent as byte 2.
static inline void spi_do_send(uint8_t const payload_length,
                               uint8_t const kind) {
  uint8_t const data_length = payload_length - 1;
  uint8_t out_crc = crc8_p93_next(0, payload_length);
  out_crc = crc8_p93_next(out_crc, kind);

  uint8_t next = (data_length > 0) ? response.b.data[0] : out_crc;
  SPI_XCHG_DISCARD(next);

  for (uint8_t i = 0; i < data_length; i++) {
    // data[i] is in flight: crc work happens during its transfer
    out_crc = crc8_p93_next(out_crc, response.b.data[i]);
    next = (i + 1 < data_length) ? response.b.data[i + 1] : out_crc;
    SPI_XCHG_DISCARD(next);
  }

  // crc is in flight
  spi_last_out_crc = out_crc;
  SPI_WAIT_OR_RETURN;
  (void)SPDR;
  spi_end(0, PROTOCOL_PAD_OK);
}

// Byte 2 (master length) was read in ISR. Byte 3 (out 0) is in flight.
static inline void spi_do_recv(uint8_t const in_length) {
  if (in_length == 0) {
    spi_reply_empty(ERROR_FRAME_LENGTH, PROTOCOL_PAD_ERROR);
    return;
  }
  if (in_length >= BUFFER_SIZE) {
    spi_reply_empty(ERROR_BUFFER_OVERFLOW, PROTOCOL_PAD_ERROR);
    return;
  }

  uint8_t crc_local = crc8_p93_next(0, in_length);
  uint8_t payload_crc = crc8_p93_next(0, 0);  // byte 2 out

  for (uint8_t i = 0; i < in_length; i++) {
    // in flight: out 0 for data byte i
    SPI_WAIT_OR_RETURN;
    rx_data[i] = SPDR;
    SPI_LOAD(0);
    crc_local = crc8_p93_next(crc_local, rx_data[i]);
    payload_crc = crc8_p93_next(payload_crc, 0);
  }

  // in flight: out 0, this exchange reads crc_remote
  SPI_WAIT_OR_RETURN;
  uint8_t const crc_remote = SPDR;
  SPI_LOAD(0);  // send 0

  // all outgoing bytes after this point are known: 0, 0, 0xff, crc_local, crc_remote
  uint8_t pc = crc8_p93_next(payload_crc, 0);  // out byte of crc_remote exchange
  pc = crc8_p93_next(pc, 0);
  pc = crc8_p93_next(pc, 0xff);
  pc = crc8_p93_next(pc, crc_local);
  pc = crc8_p93_next(pc, crc_remote);

  bool const ok = (crc_local == crc_remote);

  SPI_XCHG_DISCARD(0xff);
  SPI_XCHG_DISCARD(crc_local);
  SPI_XCHG_DISCARD(crc_remote);
  SPI_XCHG_DISCARD(pc);

  // payload_crc is shifting out; copy request now, off the critical gap
  if (ok) {
    packet_clear_fast((packet_t *)&request);
    request.h.command = rx_data[0];
    buffer_copy((buffer_t *)&request.b, rx_data + 1, in_length - 1);
    request.filled = true;
  }

  SPI_WAIT_OR_RETURN;
  (void)SPDR;
  spi_end(ok ? 0 : ERROR_INVALID_CRC, ok ? PROTOCOL_PAD_OK : PROTOCOL_PAD_ERROR);
}

// Byte 3 (local_length) is in flight.
static inline void spi_do_ack(uint8_t const local_length) {
  uint8_t const local_crc = spi_last_out_crc;  // anti-volatile
  uint8_t const payload_crc = crc8_p93_next(
      crc8_p93_next(crc8_p93_next(0, 2), local_length), local_crc);

  SPI_WAIT_OR_RETURN;
  uint8_t const remote_length = SPDR;
  SPI_LOAD(local_crc);

  SPI_WAIT_OR_RETURN;
  uint8_t const remote_crc = SPDR;
  SPI_LOAD(payload_crc);

  bool const match =
      (local_length == remote_length) && (local_crc == remote_crc);
  if (match) {
    packet_clear_fast((packet_t *)&response);
  }

  SPI_WAIT_OR_RETURN;
  (void)SPDR;
  spi_end(match ? 0 : ERROR_INVALID_ACK,
          match ? PROTOCOL_PAD_OK : PROTOCOL_PAD_ERROR);
}

ISR(PCINT0_vect) {
  if (!spi_selected()) {
    return;
  }

  // всё, что понадобится позже, считаем до первого байта
  bool const out_ready = response.filled;  // anti-volatile
  bool const req_busy = request.filled;    // anti-volatile
  uint8_t const len1 = response.b.length + 1;
  uint8_t const kind = response.h.response;

  uint8_t out_header = PROTOCOL_VERSION;
  if (out_ready) {
    bit_mask_set(out_header, PROTOCOL_FLAG_PAYLOAD);
  }
  if (req_busy) {
    bit_mask_set(out_header, PROTOCOL_FLAG_REQUEST_BUSY);
  }

  // байт 1: заголовок
  SPDR = out_header;
  DBG_TOGGLE();
  SPI_WAIT_OR_RETURN;
  uint8_t const in_header = SPDR;

  // байт 2 в ответ: выбирается по флагам заголовка мастера
  uint8_t b2;
  if (bit_mask_test(in_header, PROTOCOL_FLAG_REQUEST_BUSY)) {
    b2 = 2;
  } else if (bit_mask_test(in_header, PROTOCOL_FLAG_PAYLOAD)) {
    b2 = 0;
  } else {
    b2 = out_ready ? len1 : 0;
  }
  SPI_LOAD(b2);

  // байт 3 в ответ: тоже из заголовка, считается пока байт 2 в линии
  uint8_t b3;
  if (bit_mask_test(in_header, PROTOCOL_FLAG_REQUEST_BUSY)) {
    b3 = len1;
  } else if (bit_mask_test(in_header, PROTOCOL_FLAG_PAYLOAD)) {
    b3 = 0;
  } else {
    b3 = out_ready ? kind : 0;
  }

  // байт 2 от мастера: длина запроса
  SPI_WAIT_OR_RETURN;
  uint8_t const in_b2 = SPDR;
  SPI_LOAD(b3);

  // байт 3 уже в линии, дальше можно проверять
  uint8_t const in_mode = in_header & PROTOCOL_HEADER_FLAG_MASK;
  if ((in_header & PROTOCOL_HEADER_VERSION_MASK) != PROTOCOL_VERSION) {
    spi_reply_empty(ERROR_FRAME_HEADER, PROTOCOL_PAD_ERROR);
    return;
  }

  switch (in_mode) {
    // мастер читает ответ
    case 0:
      if (!out_ready) {
        spi_reply_empty(0, PROTOCOL_PAD_OK);
        return;
      }
      spi_do_send(b2, b3);
      return;

    // мастер шлёт запрос
    case PROTOCOL_FLAG_PAYLOAD:
      if (req_busy) {
        spi_reply_empty(ERROR_REQUEST_OVERWRITE, PROTOCOL_PAD_ERROR);
        return;
      }
      spi_do_recv(in_b2);
      return;

    // мастер подтверждает предыдущий ответ
    case PROTOCOL_FLAG_REQUEST_BUSY:
      spi_do_ack(b3);
      return;

    default:
      // не должно случаться
      spi_reply_empty(ERROR_FRAME_HEADER, PROTOCOL_PAD_ERROR);
      return;
  }
}
static inline bool spi_selected(void) {
  return !bit_mask_test(SPI_SS_PORT, _BV(SPI_SS_PIN));
}

static inline bool spi_wait(void) {
  for (;;) {
    // new byte is shifted
    if (bit_mask_test(SPSR, _BV(SPIF))) {
      return true;
    }
    // end of transmission
    if (!spi_selected()) {
      return false;
    }
  }
}

#endif  // INCLUDE_SPI_C