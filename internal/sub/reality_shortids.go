package sub

// Only the active prefix may reach subscriptions; the suffix is grace-only.
// A non-positive count preserves legacy behavior for inbounds without rotation.
func activeRealityShortIDs(value any, activeCount int) []string {
	var values []any
	switch shortIDs := value.(type) {
	case []any:
		values = shortIDs
	case []string:
		values = make([]any, len(shortIDs))
		for i := range shortIDs {
			values[i] = shortIDs[i]
		}
	default:
		return nil
	}

	limit := len(values)
	if activeCount > 0 && activeCount < limit {
		limit = activeCount
	}

	result := make([]string, 0, limit)
	for _, value := range values[:limit] {
		if shortID, ok := value.(string); ok {
			result = append(result, shortID)
		}
	}
	return result
}
