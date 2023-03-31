package manager

type origin struct {
	ID        string
	Name      string
	IPAddress string
	Disabled  bool
	Port      int64
}

type pool struct {
	ID      string
	Name    string
	Origins []origin
}

type port struct {
	AddressFamily string
	ID            string
	Name          string
	Port          int64
	Pools         []pool
}

type loadBalancer struct {
	ID    string
	Ports []port
}
