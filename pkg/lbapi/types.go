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
}

type LoadBalancer struct {
	ID    string `json:"id"`
	Ports []Port `json:"ports"`
}

type v1ResponseMetaData struct {
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

type LoadBalancerResponse struct {
	v1ResponseMetaData
	LoadBalancer LoadBalancer `json:"load_balancer"`
}

type PoolResponse struct {
	v1ResponseMetaData
	Pool Pool `json:"pool"`
}
