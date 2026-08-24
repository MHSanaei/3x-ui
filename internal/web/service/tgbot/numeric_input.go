package tgbot

// updateNumericInput applies one number-pad key: -2 clears, -1 backspaces, and 0..9 append.
// Callers retain their own validation and keyboard labels.
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
