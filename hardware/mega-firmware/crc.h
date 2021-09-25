#ifndef INCLUDE_CRC_H
#define INCLUDE_CRC_H

#include <inttypes.h>

uint8_t crc8_p93_reference(uint8_t const init, uint8_t data)
    __attribute__((const));
uint8_t crc8_p93_next(uint8_t const crc, uint8_t const data)
    __attribute__((const));
uint8_t crc8_p93_n(uint8_t const crc, uint8_t const data[], uint8_t const n);
uint8_t crc8_p93_2b(uint8_t const data1, uint8_t const data2)
    __attribute__((const));
uint8_t crc8_p93_3b(uint8_t const data1, uint8_t const data2,
                    uint8_t const data3) __attribute__((const));

#endif  // INCLUDE_CRC_H
