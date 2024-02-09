package handlers

type (
	GRPCFilterBase struct {
		FilterBase
	}
	GRPCRequestFilter struct {
		GRPCFilterBase
	}
	GRPCResponseFilter struct {
		GRPCFilterBase
	}
)
