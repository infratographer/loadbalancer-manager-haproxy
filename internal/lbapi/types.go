package lbapi

type Frontend struct {
	ID   string `json:"id"`
	Port int64  `json:"port"`
}

type Assignment struct {
	ID       string   `json:"id"`
	Frontend Frontend `json:"frontend"`
	Pools    []Pool   `json:"pools"`
}

type Origin struct {
	ID        string `json:"id"`
	Name      string `json:"display_name"`
	IPAddress string `json:"origin_target"`
	Disabled  bool   `json:"origin_disabled"`
	Port      int64  `json:"port"`
}

type Pool struct {
	ID      string   `json:"id"`
	Name    string   `json:"display_name"`
	Origins []Origin `json:"origins"`
}

type LoadBalancer struct {
	ID          string       `json:"id"`
	Assignments []Assignment `json:"assignments"`
}
