package disputes

// Classify validates a dispute request and applies the deterministic rules.
func Classify(params Params, facts Facts) (Dispute, error) {
	if err := validateParams(params); err != nil {
		return Dispute{}, err
	}

	status, code, explanation := classify(params.Reason, facts)
	return newDispute(params, status, code, explanation), nil
}
