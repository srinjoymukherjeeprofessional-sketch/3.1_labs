package main

// handlePTR encodes the target hostname as DNS labels, without compression
// pointers, so the returned bytes can be placed directly in an answer's Data.
// An absent PTR record produces no answer.
func handlePTR(target string) ([]byte, error) {
	if target == "" {
		return nil, nil
	}
	return appendName(nil, target, make(map[string]uint16))
}
