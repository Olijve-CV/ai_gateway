package handlers

func normalizeProtocol(protocol string) string {
	if protocol == "" {
		return "openai_chat"
	}
	return protocol
}

func protocolValue(protocol *string) string {
	if protocol == nil {
		return ""
	}
	return *protocol
}
