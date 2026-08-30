package text_display

// Создание паттерна для символа "стрелка вправо" (5x8)
func CreateArrowRightPatternMT() []byte {
	return []byte{
		0b00000,
		0b00100,
		0b00010,
		0b11111,
		0b00010,
		0b00100,
		0b00000,
		0b00000,
	}
}

// Создание паттерна для символа "галочка" (5x8)
func CreateCheckMarkPatternMT() []byte {
	return []byte{
		0b00000,
		0b00001,
		0b00010,
		0b10100,
		0b11000,
		0b10000,
		0b00000,
		0b00000,
	}
}

// Создание паттерна для символа "градус Цельсия" (5x8)
func CreateCelsiusPatternMT() []byte {
	return []byte{
		0b01100,
		0b10010,
		0b10010,
		0b01100,
		0b00000,
		0b00000,
		0b00000,
		0b00000,
	}
}
