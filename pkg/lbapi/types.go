package lbapi

type OriginNode struct {
	ID         string
	Name       string
	Target     string
	PortNumber int64
	Active     bool
}

type OriginEdges struct {
	Node OriginNode
}

type Origins struct {
	Edges []OriginEdges
}

type Pool struct {
	ID       string
	Name     string
	Protocol string
	Origins  Origins
}

type PortNode struct {
	ID     string
	Name   string
	Number int64
	Pools  []Pool
}

type PortEdges struct {
	Node PortNode
}

type Ports struct {
	Edges []PortEdges
}

type LoadBalancer struct {
	ID    string
	Name  string
	Ports Ports
}

type GetLoadBalancer struct {
	LoadBalancer LoadBalancer `graphql:"loadBalancer(id: $id)"`
}

// Readable version of the above:
// type GetLoadBalancer struct {
// 	LoadBalancer struct {
// 		ID    string
// 		Name  string
// 		Ports struct {
// 			Edges []struct {
// 				Node struct {
// 					Name   string
// 					Number int64
// 					Pools  []struct {
// 						Name     string
// 						Protocol string
// 						Origins  struct {
// 							Edges []struct {
// 								Node struct {
// 									Name       string
// 									Target     string
// 									PortNumber int64
// 									Active     bool
// 								}
// 							}
// 						}
// 					}
// 				}
// 			}
// 		}
// 	} `graphql:"loadBalancer(id: $id)"`
// }
