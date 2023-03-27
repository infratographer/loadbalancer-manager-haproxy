package lbapi

type Port struct {
	ID   string `json:"id"`
	Port int64  `json:"port"`
}

type Assignment struct {
	ID    string `json:"id"`
	Port  Port   `json:"port"`
	Pools []Pool `json:"pools"`
}

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

type LoadBalancer struct {
	ID          string       `json:"id"`
	Assignments []Assignment `json:"assignments"`
}
