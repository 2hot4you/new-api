package claude

func NormalizeCacheCreationSplit(totalTokens int, tokens5m int, tokens1h int) (int, int) {
	totalTokens = max(totalTokens, 0)
	tokens5m = max(tokens5m, 0)
	tokens1h = max(tokens1h, 0)
	// Preserve an upstream split even when it exceeds the reported total.
	// Subtract only while both operands are ordered, so a
	// hostile pair near MaxInt cannot wrap the computed five-minute count.
	if totalTokens <= tokens5m {
		return tokens5m, tokens1h
	}
	if remaining := totalTokens - tokens5m; remaining <= tokens1h {
		return tokens5m, tokens1h
	}
	return totalTokens - tokens1h, tokens1h
}
