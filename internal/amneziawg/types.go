package amneziawg

// ServerConfig holds the AmneziaWG server configuration parameters.
type ServerConfig struct {
	PrivateKey    string `json:"privateKey"`
	PublicKey     string `json:"publicKey"`
	PSK           string `json:"psk"`
	Jc            int    `json:"jc"`
	Jmin          int    `json:"jmin"`
	Jmax          int    `json:"jmax"`
	S1            int    `json:"s1"`
	S2            int    `json:"s2"`
	S3            int    `json:"s3"`
	S4            int    `json:"s4"`
	H1            string `json:"h1"`
	H2            string `json:"h2"`
	H3            string `json:"h3"`
	H4            string `json:"h4"`
	SubnetIP      string `json:"subnetIp"`
	SubnetCIDR    int    `json:"subnetCidr"`
	ServerPort    int    `json:"serverPort"`
	PrimaryDNS    string `json:"primaryDns"`
	SecondaryDNS  string `json:"secondaryDns"`
}

// ClientSettings holds the per-client configuration for AmneziaWG.
type ClientSettings struct {
	Email       string `json:"email"`
	PrivateKey  string `json:"privateKey"`
	PublicKey   string `json:"publicKey"`
	PresharedKey string `json:"presharedKey"`
	AssignedIP  string `json:"assignedIp"`
	Enable      bool   `json:"enable"`
	TotalGB     int64  `json:"totalGB"`
	ExpiryTime  int64  `json:"expiryTime"`
}

// SettingsInbound is the top-level settings JSON structure for an AWG inbound.
type SettingsInbound struct {
	Server  *ServerConfig   `json:"server"`
	Clients []ClientSettings `json:"clients"`
}
