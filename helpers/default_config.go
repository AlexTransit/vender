package helpers

func ConfigDefaultInt(InInt int, valueIfIntZero int) int {
	if InInt == 0 {
		return valueIfIntZero
	}
	return InInt
}

func ConfigDefaultStr(inString string, valueIfStringBlank string) string {
	if inString == "" {
		return valueIfStringBlank
	}
	return inString
}
