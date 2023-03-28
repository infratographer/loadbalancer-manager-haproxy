package lbapi

type Origin struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IPAddress string `json:"origin_target"`
	Disabled  bool   `json:"origin_disabled"`
	Port      int64  `json:"port"`
}

type Pool struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Origins []Origin `json:"origins"`
}

type Port struct {
	AddressFamily string   `json:"address_family"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Port          int64    `json:"port"`
	Pools         []string `json:"pools"`
	PoolData      []Pool
}

type LoadBalancer struct {
	ID    string `json:"id"`
	Ports []Port `json:"ports"`
}
