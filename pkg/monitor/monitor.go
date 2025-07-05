package monitor

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"hpa-monitor/pkg/logger"
)

// HPAMonitor handles HPA monitoring logic
type HPAMonitor struct {
	client    kubernetes.Interface
	tolerance float64
}

// NewHPAMonitor creates a new HPA monitor instance
func NewHPAMonitor(client kubernetes.Interface) *HPAMonitor {
	return &HPAMonitor{
		client:    client,
		tolerance: 0.1, // 10% tolerance
	}
}

// GetHPAStatus retrieves the current status of all HPAs in the cluster
func (hm *HPAMonitor) GetHPAStatus(ctx context.Context) ([]HPAStatus, error) {
	log := logger.GetLogger()
	
	hpaList, err := hm.client.AutoscalingV2().HorizontalPodAutoscalers("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.WithError(err).Error("Failed to list HPA resources")
		return nil, err
	}

	var hpaStatuses []HPAStatus
	log.WithField("count", len(hpaList.Items)).Info("Processing HPAs")
	
	for _, hpa := range hpaList.Items {
		status := hm.buildHPAStatus(&hpa)
		hpaStatuses = append(hpaStatuses, status)
		
		// Log summary for this HPA
		currentVal := "N/A"
		targetVal := "N/A"
		if status.PrimaryMetricCurrent != nil {
			currentVal = *status.PrimaryMetricCurrent
		}
		if status.PrimaryMetricTarget != nil {
			targetVal = *status.PrimaryMetricTarget
		}
		
		log.WithFields(logger.Fields{
			"namespace":       hpa.Namespace,
			"name":           hpa.Name,
			"metric":         status.PrimaryMetricName,
			"current":        currentVal,
			"target":         targetVal,
			"current_replicas": status.CurrentReplicas,
			"desired_replicas": status.DesiredReplicas,
			"min_replicas":   status.MinReplicas,
			"max_replicas":   status.MaxReplicas,
		}).Debug("HPA status processed")
	}

	return hpaStatuses, nil
}

// buildHPAStatus builds HPAStatus from Kubernetes HPA resource
func (hm *HPAMonitor) buildHPAStatus(hpa *autoscalingv2.HorizontalPodAutoscaler) HPAStatus {
	status := HPAStatus{
		Name:            hpa.Name,
		Namespace:       hpa.Namespace,
		MinReplicas:     *hpa.Spec.MinReplicas,
		MaxReplicas:     hpa.Spec.MaxReplicas,
		CurrentReplicas: hpa.Status.CurrentReplicas,
		DesiredReplicas: hpa.Status.DesiredReplicas,
		Ready:           len(hpa.Status.Conditions) > 0,
	}

	// Set tolerance value
	status.Tolerance = hm.tolerance

	// Apply tolerance to min/max replicas
	status.ToleranceAdjustedMin = int32(math.Ceil(float64(status.MinReplicas) * (1 - hm.tolerance)))
	status.ToleranceAdjustedMax = int32(math.Floor(float64(status.MaxReplicas) * (1 + hm.tolerance)))

	// Extract CPU utilization metrics
	hm.extractMetrics(hpa, &status)

	// Check scaling conditions
	hm.checkScalingConditions(hpa, &status)

	// Check last scale time
	hm.setLastScaleTime(hpa, &status)

	// Check scaling stabilization
	hm.checkScalingStabilization(hpa, &status)

	// Fetch events for the HPA
	hm.fetchEvents(hpa, &status)

	return status
}

// extractMetrics extracts all types of metrics from HPA
func (hm *HPAMonitor) extractMetrics(hpa *autoscalingv2.HorizontalPodAutoscaler, status *HPAStatus) {
	
	// Extract target metrics (first metric becomes primary)
	for i, metric := range hpa.Spec.Metrics {
		if i == 0 { // Use first metric as primary
			switch metric.Type {
			case autoscalingv2.ResourceMetricSourceType:
				status.PrimaryMetricName = string(metric.Resource.Name)
				if metric.Resource.Target.AverageUtilization != nil {
					target := fmt.Sprintf("%d%%", *metric.Resource.Target.AverageUtilization)
					status.PrimaryMetricTarget = &target
				}
				// Still handle CPU for backwards compatibility
				if metric.Resource.Name == "cpu" && metric.Resource.Target.AverageUtilization != nil {
					status.TargetCPUUtilization = metric.Resource.Target.AverageUtilization
				}
			case autoscalingv2.ExternalMetricSourceType:
				status.PrimaryMetricName = metric.External.Metric.Name
				if metric.External.Target.AverageValue != nil {
					target := metric.External.Target.AverageValue.String()
					status.PrimaryMetricTarget = &target
				} else if metric.External.Target.Value != nil {
					target := metric.External.Target.Value.String()
					status.PrimaryMetricTarget = &target
				}
			case autoscalingv2.ObjectMetricSourceType:
				status.PrimaryMetricName = metric.Object.Metric.Name
				if metric.Object.Target.AverageValue != nil {
					target := metric.Object.Target.AverageValue.String()
					status.PrimaryMetricTarget = &target
				} else if metric.Object.Target.Value != nil {
					target := metric.Object.Target.Value.String()
					status.PrimaryMetricTarget = &target
				}
			}
		}
	}

	// Extract current metrics
	for i, metric := range hpa.Status.CurrentMetrics {
		if i == 0 { // Use first metric as primary
			switch metric.Type {
			case autoscalingv2.ResourceMetricSourceType:
				if metric.Resource.Current.AverageUtilization != nil {
					current := fmt.Sprintf("%d%%", *metric.Resource.Current.AverageUtilization)
					status.PrimaryMetricCurrent = &current
				}
				// Still handle CPU for backwards compatibility
				if metric.Resource.Name == "cpu" && metric.Resource.Current.AverageUtilization != nil {
					status.CurrentCPUUtilization = metric.Resource.Current.AverageUtilization
				}
			case autoscalingv2.ExternalMetricSourceType:
				if metric.External.Current.AverageValue != nil {
					current := metric.External.Current.AverageValue.String()
					status.PrimaryMetricCurrent = &current
				} else if metric.External.Current.Value != nil {
					current := metric.External.Current.Value.String()
					status.PrimaryMetricCurrent = &current
				}
			case autoscalingv2.ObjectMetricSourceType:
				if metric.Object.Current.AverageValue != nil {
					current := metric.Object.Current.AverageValue.String()
					status.PrimaryMetricCurrent = &current
				} else if metric.Object.Current.Value != nil {
					current := metric.Object.Current.Value.String()
					status.PrimaryMetricCurrent = &current
				}
			}
		}
	}

	// Calculate ratio (Current/Target) for all metric types
	ratio := calculateRatio(status)
	if ratio != nil {
		status.Ratio = ratio
	}

	// Set default primary metric name if none found
	if status.PrimaryMetricName == "" {
		status.PrimaryMetricName = "Unknown"
	}

}

// checkScalingConditions checks HPA conditions for readiness
func (hm *HPAMonitor) checkScalingConditions(hpa *autoscalingv2.HorizontalPodAutoscaler, status *HPAStatus) {
	for _, condition := range hpa.Status.Conditions {
		if condition.Type == autoscalingv2.ScalingActive {
			status.Ready = condition.Status == "True"
		}
	}
}

// setLastScaleTime sets the last scale time in the status
func (hm *HPAMonitor) setLastScaleTime(hpa *autoscalingv2.HorizontalPodAutoscaler, status *HPAStatus) {
	if hpa.Status.LastScaleTime != nil {
		lastScaleTime := hpa.Status.LastScaleTime.Format(time.RFC3339)
		status.LastScaleTime = &lastScaleTime
	}
}

// checkScalingStabilization checks if scaling is stabilized
func (hm *HPAMonitor) checkScalingStabilization(hpa *autoscalingv2.HorizontalPodAutoscaler, status *HPAStatus) {
	now := time.Now()
	if hpa.Status.LastScaleTime != nil {
		timeSinceLastScale := now.Sub(hpa.Status.LastScaleTime.Time)
		status.ScaleUpStabilized = timeSinceLastScale > 3*time.Minute
		status.ScaleDownStabilized = timeSinceLastScale > 5*time.Minute
	} else {
		status.ScaleUpStabilized = true
		status.ScaleDownStabilized = true
	}
}

// SetTolerance sets the tolerance percentage (0.0 to 1.0)
func (hm *HPAMonitor) SetTolerance(tolerance float64) {
	log := logger.GetLogger()
	
	if tolerance >= 0.0 && tolerance <= 1.0 {
		hm.tolerance = tolerance
		log.WithFields(logger.Fields{
			"tolerance": tolerance,
			"percentage": tolerance * 100,
		}).Info("Tolerance updated")
	} else {
		log.WithField("tolerance", tolerance).Warn("Invalid tolerance value. Must be between 0.0 and 1.0")
	}
}

// GetTolerance returns the current tolerance setting
func (hm *HPAMonitor) GetTolerance() float64 {
	return hm.tolerance
}

// fetchEvents fetches events related to the HPA
func (hm *HPAMonitor) fetchEvents(hpa *autoscalingv2.HorizontalPodAutoscaler, status *HPAStatus) {
	log := logger.GetLogger()
	ctx := context.Background()
	
	events, err := hm.client.CoreV1().Events(hpa.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + hpa.Name,
	})
	if err != nil {
		log.WithFields(logger.Fields{
			"namespace": hpa.Namespace,
			"name":      hpa.Name,
		}).WithError(err).Error("Failed to fetch events for HPA")
		status.Events = []Event{}
		return
	}

	log.WithFields(logger.Fields{
		"namespace":   hpa.Namespace,
		"name":        hpa.Name,
		"event_count": len(events.Items),
	}).Debug("Fetched events for HPA")

	var hpaEvents []Event
	for _, event := range events.Items {
		hpaEvent := Event{
			Type:    event.Type,
			Reason:  event.Reason,
			Message: event.Message,
			Count:   event.Count,
		}
		
		if event.FirstTimestamp.Time.IsZero() {
			hpaEvent.FirstTimestamp = "Unknown"
		} else {
			hpaEvent.FirstTimestamp = event.FirstTimestamp.Format(time.RFC3339)
		}
		
		if event.LastTimestamp.Time.IsZero() {
			hpaEvent.LastTimestamp = "Unknown"
		} else {
			hpaEvent.LastTimestamp = event.LastTimestamp.Format(time.RFC3339)
		}
		
		hpaEvents = append(hpaEvents, hpaEvent)
	}

	status.Events = hpaEvents
}

// calculateRatio calculates the ratio between current and target metrics
func calculateRatio(status *HPAStatus) *float64 {
	// First try CPU metrics for backwards compatibility
	if status.TargetCPUUtilization != nil && *status.TargetCPUUtilization > 0 {
		currentCPU := int32(0) // Default to 0 if nil
		if status.CurrentCPUUtilization != nil {
			currentCPU = *status.CurrentCPUUtilization
		}
		ratio := float64(currentCPU) / float64(*status.TargetCPUUtilization)
		return &ratio
	}

	// Try primary metrics with direct percentage parsing
	if status.PrimaryMetricCurrent != nil && status.PrimaryMetricTarget != nil {
		current := *status.PrimaryMetricCurrent
		target := *status.PrimaryMetricTarget
		
		// Direct percentage parsing for common cases like "0%" and "60%"
		if strings.HasSuffix(current, "%") && strings.HasSuffix(target, "%") {
			currentNum := strings.TrimSuffix(current, "%")
			targetNum := strings.TrimSuffix(target, "%")
			
			if currentVal, err := strconv.ParseFloat(currentNum, 64); err == nil {
				if targetVal, err := strconv.ParseFloat(targetNum, 64); err == nil && targetVal > 0 {
					ratio := currentVal / targetVal
					return &ratio
				}
			}
		}
		
		// Try advanced parsing for other metric types
		currentValue := parseMetricValue(current)
		targetValue := parseMetricValue(target)
		
		if currentValue != nil && targetValue != nil && *targetValue > 0 {
			ratio := *currentValue / *targetValue
			return &ratio
		}
	}

	return nil
}

// parseMetricValue parses a metric value string and returns the numeric value
func parseMetricValue(value string) *float64 {
	// Remove common units and parse
	value = strings.TrimSpace(value)
	
	// Handle percentage values
	if strings.HasSuffix(value, "%") {
		value = strings.TrimSuffix(value, "%")
	}
	
	// Handle resource units (Ki, Mi, Gi, etc.)
	value = strings.TrimSuffix(value, "Ki")
	value = strings.TrimSuffix(value, "Mi")
	value = strings.TrimSuffix(value, "Gi")
	value = strings.TrimSuffix(value, "Ti")
	value = strings.TrimSuffix(value, "Pi")
	
	// Handle metric units (m for milli, k for kilo, etc.)
	multiplier := 1.0
	if strings.HasSuffix(value, "m") {
		value = strings.TrimSuffix(value, "m")
		multiplier = 0.001
	} else if strings.HasSuffix(value, "k") {
		value = strings.TrimSuffix(value, "k")
		multiplier = 1000.0
	}
	
	// Try to parse the numeric value
	if parsedValue, err := strconv.ParseFloat(value, 64); err == nil {
		result := parsedValue * multiplier
		return &result
	}
	
	return nil
}