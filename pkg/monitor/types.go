package monitor

// HPAStatus represents the status of a Horizontal Pod Autoscaler
type HPAStatus struct {
	Name                    string  `json:"name"`
	Namespace               string  `json:"namespace"`
	MinReplicas             int32   `json:"minReplicas"`
	MaxReplicas             int32   `json:"maxReplicas"`
	CurrentReplicas         int32   `json:"currentReplicas"`
	DesiredReplicas         int32   `json:"desiredReplicas"`
	CurrentCPUUtilization   *int32   `json:"currentCPUUtilization"`
	TargetCPUUtilization    *int32   `json:"targetCPUUtilization"`
	PrimaryMetricName       string   `json:"primaryMetricName"`
	PrimaryMetricCurrent    *string  `json:"primaryMetricCurrent"`
	PrimaryMetricTarget     *string  `json:"primaryMetricTarget"`
	Ratio                   *float64 `json:"ratio"`
	Tolerance               float64 `json:"tolerance"`
	ToleranceAdjustedMin    int32   `json:"toleranceAdjustedMin"`
	ToleranceAdjustedMax    int32   `json:"toleranceAdjustedMax"`
	LastScaleTime           *string `json:"lastScaleTime"`
	Ready                   bool    `json:"ready"`
	ScaleUpStabilized       bool    `json:"scaleUpStabilized"`
	ScaleDownStabilized     bool    `json:"scaleDownStabilized"`
	Events                  []Event `json:"events"`
}

// Event represents a Kubernetes event
type Event struct {
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	FirstTimestamp string `json:"firstTimestamp"`
	LastTimestamp  string `json:"lastTimestamp"`
	Count          int32  `json:"count"`
}