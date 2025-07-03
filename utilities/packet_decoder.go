package utilities

func DecodeIdentifier(decodedPayload []int) int {
	return decodedPayload[3]<<8 | decodedPayload[4]
}

func DecodeSecret(secretPayload []int) []byte {
	decodeResult := make([]byte, 24)
	runningValue := 0

	for index := 0; index < 24; index++ {
		decodedChar1 := DecodeSecretChar(NormalizeChar(secretPayload[17+index*2]), 98, index)
		decodedChar2 := DecodeSecretChar(NormalizeChar(secretPayload[18+index*2]), 115, index)

		interim := (decodedChar1 << 4) | decodedChar2
		asciiOffset := Conditional(interim >= 97, 97, 65)

		interim -= asciiOffset
		if index == 0 {
			runningValue = 2 + interim
		}

		decodeResult[index] = byte((interim+runningValue)%26 + asciiOffset)
		runningValue += 3 + interim
	}

	return decodeResult
}

func DecodeSecretChar(charCode, shift, index int) int {
	decodedCharCode := (charCode - shift - index*34) % 26
	if decodedCharCode < 0 {
		decodedCharCode += 26
	}
	return decodedCharCode
}

func NormalizeChar(charCode int) int {
	if charCode <= 96 {
		return charCode + 32
	}
	return charCode
}
