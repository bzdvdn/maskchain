package routing

// @sk-task 70-routing-engine#T1.1: Create Route entity (AC-001)
type Route struct {
	Model     string
	Providers []string
}

func NewRoute(model string, providers []string) Route {
	return Route{Model: model, Providers: providers}
}
