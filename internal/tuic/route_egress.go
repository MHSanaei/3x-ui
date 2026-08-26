package tuic

const TUICBasePort = 63200

func SOCKSPortForInbound(inboundID int) int {
	return TUICBasePort + inboundID
}
