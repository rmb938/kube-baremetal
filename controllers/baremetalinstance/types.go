package baremetalinstance

type NetworkDataLink struct {
	ID         string   `json:"id"`
	MAC        string   `json:"ethernet_mac_address"`
	Type       string   `json:"type"`
	BondMode   string   `json:"bond_mode,omitempty"`
	BondMiiMon int      `json:"bond_miimon,omitempty"`
	BondLinks  []string `json:"bond_links,omitempty"`
}

type NetworkDataNetwork struct {
	Link        string   `json:"link"`
	Type        string   `json:"type"`
	IPAddress   string   `json:"ip_address"`
	Netmask     string   `json:"netmask"`
	Gateway     string   `json:"gateway,omitempty"`
	Nameservers []string `json:"dns_nameservers,omitempty"`
	Search      []string `json:"dns_search,omitempty"`
}

type NetworkData struct {
	Links    []NetworkDataLink    `json:"links"`
	Networks []NetworkDataNetwork `json:"networks"`
}
