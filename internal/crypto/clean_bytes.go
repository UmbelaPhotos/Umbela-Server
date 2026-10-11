package crypto

func CleanBytes(messyBytes ...[]byte) {
	for _, bytesArray := range messyBytes {
		for j := range bytesArray {
			bytesArray[j] = 0
		}
	}
}