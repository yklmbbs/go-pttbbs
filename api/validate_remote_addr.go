package api

func isValidRemoteAddr(remoteAddr string) bool {
	switch remoteAddr {
	case "":
		return false
	default:
		return true
	}
}
