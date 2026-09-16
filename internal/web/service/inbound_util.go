package service

import "strings"

// sqliteMaxVars is a safe ceiling for the number of bind parameters in a
// single SQL statement. SQLite's SQLITE_MAX_VARIABLE_NUMBER is 999 on builds
// before 3.32 and 32766 after; staying under 999 keeps queries portable
// across forks/old binaries and also bounds per-query memory on truly large
// installs (>32k clients) where even modern SQLite would refuse a single IN.
const sqliteMaxVars = 900

// normalizeSubSortIndex maps omitted/zero to 1; explicit negatives are kept
// so a primary inbound can sort ahead of the default.
func normalizeSubSortIndex(v int) int {
	if v == 0 {
		return 1
	}
	return v
}

// uniqueNonEmptyStrings returns a deduplicated copy of in with empty strings
// removed, preserving the order of first occurrence.
func uniqueNonEmptyStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// trimmedUniqueEmails trims each address before deduplicating, so the bulk
// client operations treat " a@x " and "a@x" as the same row.
func trimmedUniqueEmails(in []string) []string {
	trimmed := make([]string, 0, len(in))
	for _, e := range in {
		trimmed = append(trimmed, strings.TrimSpace(e))
	}
	return uniqueNonEmptyStrings(trimmed)
}

// uniqueInts returns a deduplicated copy of in, preserving order of first occurrence.
func uniqueInts(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// chunkStrings splits s into consecutive sub-slices of at most size elements.
// Returns nil for an empty input or non-positive size.
func chunkStrings(s []string, size int) [][]string {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	out := make([][]string, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := min(i+size, len(s))
		out = append(out, s[i:end])
	}
	return out
}

// chunkInts splits s into consecutive sub-slices of at most size elements.
// Returns nil for an empty input or non-positive size.
func chunkInts(s []int, size int) [][]int {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	out := make([][]int, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		end := min(i+size, len(s))
		out = append(out, s[i:end])
	}
	return out
}
