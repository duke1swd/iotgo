package main

// Validates that an ID conforms to the Homie standard.

func validDevice(inputId string) bool {
	if len(inputId) < 1 {
		return false
	}

	bytes := []byte(inputId)

	if bytes[0] == '-' {
		return false
	}

	for _, b := range bytes {
		if (b < 'a' || b > 'z') &&
			(b < 'A' || b > 'Z') &&
			(b < '0' || b > '9') &&
			b != '-' {
			return false
		}
	}

	return true
}
