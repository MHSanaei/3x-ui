package tgbot

// updateNumericInput applies one key from the shared inline number pad.
// Key -2 clears the value, -1 removes the last decimal digit, and 0..9 append
// a digit. Callers retain their own validation and keyboard labels.
func updateNumericInput(value, key int) int {
	switch key {
	case -2:
		return 0
	case -1:
		if value > 0 {
			return value / 10
		}
		return value
	default:
		return value*10 + key
	}
}
