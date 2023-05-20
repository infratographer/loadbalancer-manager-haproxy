package lbapi

type GetLoadBalancer struct {
	LoadBalancer struct {
		ID    string
		Name  string
		Ports struct {
			Edges []struct {
				Node struct {
					Name   string
					Number int64
					Pools  []struct {
						Name     string
						Protocol string
						Origins  struct {
							Edges []struct {
								Node struct {
									Name       string
									Target     string
									PortNumber int64
									Active     bool
								}
							}
						}
					}
				}
			}
		}
	} `graphql:"loadBalancer(id: $id)"`
}
