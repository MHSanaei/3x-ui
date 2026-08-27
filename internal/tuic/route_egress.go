package tuic

const SOCKSBasePort = 63200
const TUICBasePort = SOCKSBasePort

func SOCKSPortForInbound(inboundID int) int {
	return SOCKSBasePort + inboundID
}
