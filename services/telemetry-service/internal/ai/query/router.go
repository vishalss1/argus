package query

import (
	"fmt"

	"github.com/vishalss1/argus/telemetry/internal/ai/operations"
)

type Router struct {
	fleet      QueryHandler
	device     QueryHandler
	incident   QueryHandler
	historical QueryHandler
}

func NewRouter(
	fleet QueryHandler,
	device QueryHandler,
	incident QueryHandler,
	historical QueryHandler,
) *Router {
	return &Router{
		fleet:      fleet,
		device:     device,
		incident:   incident,
		historical: historical,
	}
}

func (r *Router) Route(intent operations.Intent) (QueryHandler, error) {
	switch intent {
	case operations.IntentFleetSummary, operations.IntentDeviceComparison:
		return r.fleet, nil
	case operations.IntentDeviceSummary, operations.IntentRootCauseAnalysis, operations.IntentRemediation:
		return r.device, nil
	case operations.IntentIncidentLookup:
		return r.incident, nil
	case operations.IntentUnknown:
		return r.historical, nil
	default:
		return nil, fmt.Errorf("no handler registered for intent %q", intent)
	}
}

func (r *Router) Validate() error {
	intents := []operations.Intent{
		operations.IntentFleetSummary,
		operations.IntentDeviceComparison,
		operations.IntentDeviceSummary,
		operations.IntentRootCauseAnalysis,
		operations.IntentRemediation,
		operations.IntentIncidentLookup,
		operations.IntentUnknown,
	}

	for _, intent := range intents {
		if _, err := r.Route(intent); err != nil {
			return err
		}
	}
	return nil
}
