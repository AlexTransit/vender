package text_display

import "fmt"

// ProgramCustomCharMT16S2R программирует пользовательский символ для MT-16S2R
// charCode - код символа (0-7)
// pattern - массив из 8 байт, каждый байт - строка символа (5x8)
// Возвращает ошибку, если параметры некорректны
func (td *TextDisplay) ProgramCustomCharMT16S2R(charCode byte, pattern []byte) error {
	td.mu.Lock()
	defer td.mu.Unlock()

	if td.dev == nil {
		return fmt.Errorf("device not initialized")
	}

	// Проверка кода символа (0-7)
	if charCode > 7 {
		return fmt.Errorf("char code must be 0-7, got %d", charCode)
	}

	// Для MT-16S2R всегда 8 строк (5x8)
	if len(pattern) != 8 {
		return fmt.Errorf("pattern must be 8 bytes for 5x8 font, got %d", len(pattern))
	}

	// Проверка каждой строки (используются только 5 младших бит)
	for i, rowData := range pattern {
		if rowData > 0x1F {
			return fmt.Errorf("row %d: data must be 0-31 (5 bits), got %d", i, rowData)
		}
	}

	// Вычисляем адрес CGRAM для символа
	// Для 5x8: адрес = charCode * 8
	cgramAddr := byte(charCode * 8)

	// 1. Установка адреса CGRAM
	// Команда: 0x40 | cgramAddr
	td.sendCommand(0x40 | cgramAddr)

	// 2. Запись данных паттерна в CGRAM
	for _, rowData := range pattern {
		td.writeData(rowData)
	}

	// 3. Возврат в режим DDRAM
	td.sendCommand(0x80) // Set DDRAM address to 0

	// // Обновляем состояние дисплея
	// td.state.L1 = td.Translate(td.line[0])
	// td.state.L2 = td.Translate(td.line[1])
	// td.flush()

	td.log.NoticeF("Custom character %d programmed successfully for MT-16S2R", charCode)
	return nil
}
