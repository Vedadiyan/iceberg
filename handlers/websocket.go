package handlers

type (
	WSFilterBase struct {
		FilterBase
		Method string
	}
	WSRequestFilter struct {
		WSFilterBase
	}
	WSResponseFilter struct {
		WSFilterBase
	}
)
