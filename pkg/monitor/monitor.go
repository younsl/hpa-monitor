package monitor

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
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
		hm.logHPAStatus(&hpa, &status)
	}

	return hpaStatuses, nil
}

// buildHPAStatus builds HPAStatus from Kubernetes HPA resource
func (hm *HPAMonitor) buildHPAStatus(hpa *autoscalingv2.HorizontalPodAutoscaler) HPAStatus {
	status := hm.initializeHPAStatus(hpa)
	
	hm.extractMetrics(hpa, &status)
	hm.checkScalingConditions(hpa, &status)
	hm.setLastScaleTime(hpa, &status)
	hm.checkScalingStabilization(hpa, &status)
	hm.fetchEvents(hpa, &status)

	return status
}

// initializeHPAStatus initializes basic HPA status fields
func (hm *HPAMonitor) initializeHPAStatus(hpa *autoscalingv2.HorizontalPodAutoscaler) HPAStatus {
	minReplicas := int32(1)
	if hpa.Spec.MinReplicas != nil {
		minReplicas = *hpa.Spec.MinReplicas
	}
	
	return HPAStatus{
		Name:            hpa.Name,
		Namespace:       hpa.Namespace,
		MinReplicas:     minReplicas,
		MaxReplicas:     hpa.Spec.MaxReplicas,
		CurrentReplicas: hpa.Status.CurrentReplicas,
		DesiredReplicas: hpa.Status.DesiredReplicas,
		Ready:           len(hpa.Status.Conditions) > 0,
		Tolerance:       hm.tolerance,
		ToleranceAdjustedMin: int32(math.Ceil(float64(minReplicas) * (1 - hm.tolerance))),
		ToleranceAdjustedMax: int32(math.Floor(float64(hpa.Spec.MaxReplicas) * (1 + hm.tolerance))),
	}
}

// logHPAStatus logs HPA processing summary
func (hm *HPAMonitor) logHPAStatus(hpa *autoscalingv2.HorizontalPodAutoscaler, status *HPAStatus) {
	log := logger.GetLogger()
	
	currentVal := hm.getStringValue(status.PrimaryMetricCurrent)
	targetVal := hm.getStringValue(status.PrimaryMetricTarget)
	
	log.WithFields(logger.Fields{
		"namespace":        hpa.Namespace,
		"name":            hpa.Name,
		"metric":          status.PrimaryMetricName,
		"current":         currentVal,
		"target":          targetVal,
		"current_replicas": status.CurrentReplicas,
		"desired_replicas": status.DesiredReplicas,
		"min_replicas":    status.MinReplicas,
		"max_replicas":    status.MaxReplicas,
	}).Debug("HPA status processed")
}

// getStringValue safely gets string value from pointer
func (hm *HPAMonitor) getStringValue(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return "N/A"
}

// extractMetrics extracts all types of metrics from HPA
func (hm *HPAMonitor) extractMetrics(hpa *autoscalingv2.HorizontalPodAutoscaler, status *HPAStatus) {
	// Extract primary metric (first metric in spec)
	if len(hpa.Spec.Metrics) > 0 {
		hm.extractTargetMetric(hpa.Spec.Metrics[0], status)
	}

	// Extract current metric values
	if len(hpa.Status.CurrentMetrics) > 0 {
		hm.extractCurrentMetric(hpa.Status.CurrentMetrics[0], status)
	}

	// Calculate ratio and set defaults
	hm.finalizeMetrics(status)
}

// extractTargetMetric extracts target metric information
func (hm *HPAMonitor) extractTargetMetric(metric autoscalingv2.MetricSpec, status *HPAStatus) {
	switch metric.Type {
	case autoscalingv2.ResourceMetricSourceType:
		hm.handleResourceTarget(metric.Resource, status)
	case autoscalingv2.ContainerResourceMetricSourceType:
		hm.handleContainerResourceTarget(metric.ContainerResource, status)
	case autoscalingv2.ExternalMetricSourceType:
		hm.handleExternalTarget(metric.External, status)
	case autoscalingv2.ObjectMetricSourceType:
		hm.handleObjectTarget(metric.Object, status)
	}
}

// extractCurrentMetric extracts current metric values
func (hm *HPAMonitor) extractCurrentMetric(metric autoscalingv2.MetricStatus, status *HPAStatus) {
	switch metric.Type {
	case autoscalingv2.ResourceMetricSourceType:
		hm.handleResourceCurrent(metric.Resource, status)
	case autoscalingv2.ContainerResourceMetricSourceType:
		hm.handleContainerResourceCurrent(metric.ContainerResource, status)
	case autoscalingv2.ExternalMetricSourceType:
		hm.handleExternalCurrent(metric.External, status)
	case autoscalingv2.ObjectMetricSourceType:
		hm.handleObjectCurrent(metric.Object, status)
	}
}

// finalizeMetrics calculates ratio and sets default values
func (hm *HPAMonitor) finalizeMetrics(status *HPAStatus) {
	if ratio := calculateRatio(status); ratio != nil {
		status.Ratio = ratio
	}

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
		hpaEvents = append(hpaEvents, hm.convertKubernetesEvent(event))
	}

	status.Events = hpaEvents
}

// convertKubernetesEvent converts Kubernetes event to internal Event struct
func (hm *HPAMonitor) convertKubernetesEvent(event v1.Event) Event {
	return Event{
		Type:    event.Type,
		Reason:  event.Reason,
		Message: event.Message,
		Count:   event.Count,
		FirstTimestamp: hm.formatTimestamp(event.FirstTimestamp.Time),
		LastTimestamp:  hm.formatTimestamp(event.LastTimestamp.Time),
	}
}

// formatTimestamp formats timestamp, returns "Unknown" for zero time
func (hm *HPAMonitor) formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "Unknown"
	}
	return t.Format(time.RFC3339)
}

// normalizeMetricValue normalizes metric values (converts 1000m to 1)
// CPU metrics are excluded from normalization
func (hm *HPAMonitor) normalizeMetricValue(value string, metricName string) string {
	// Don't normalize percentage values or CPU metrics
	if strings.HasSuffix(value, "%") || strings.Contains(strings.ToLower(metricName), "cpu") {
		return value
	}
	
	// Check if value ends with 'm' (milli unit)
	if strings.HasSuffix(value, "m") {
		valueStr := strings.TrimSuffix(value, "m")
		if parsedValue, err := strconv.ParseFloat(valueStr, 64); err == nil {
			// Convert milli to base unit (1000m = 1)
			normalizedValue := parsedValue / 1000.0
			
			// Format with appropriate precision
			if normalizedValue == float64(int64(normalizedValue)) {
				return fmt.Sprintf("%.0f", normalizedValue)
			} else if normalizedValue >= 10 {
				return fmt.Sprintf("%.1f", normalizedValue)
			} else {
				return fmt.Sprintf("%.2f", normalizedValue)
			}
		}
	}
	
	// Return original value if not a milli unit or parsing failed
	return value
}

// Resource metric handlers
func (hm *HPAMonitor) handleResourceTarget(resource *autoscalingv2.ResourceMetricSource, status *HPAStatus) {
	status.PrimaryMetricName = string(resource.Name)
	if resource.Target.AverageUtilization != nil {
		target := fmt.Sprintf("%d%%", *resource.Target.AverageUtilization)
		status.PrimaryMetricTarget = &target
		// Backwards compatibility for CPU
		if resource.Name == "cpu" {
			status.TargetCPUUtilization = resource.Target.AverageUtilization
		}
	}
}

func (hm *HPAMonitor) handleResourceCurrent(resource *autoscalingv2.ResourceMetricStatus, status *HPAStatus) {
	if resource.Current.AverageUtilization != nil {
		current := fmt.Sprintf("%d%%", *resource.Current.AverageUtilization)
		status.PrimaryMetricCurrent = &current
		// Backwards compatibility for CPU
		if resource.Name == "cpu" {
			status.CurrentCPUUtilization = resource.Current.AverageUtilization
		}
	}
}

// Container resource metric handlers
func (hm *HPAMonitor) handleContainerResourceTarget(containerResource *autoscalingv2.ContainerResourceMetricSource, status *HPAStatus) {
	status.PrimaryMetricName = string(containerResource.Name)
	if containerResource.Target.AverageUtilization != nil {
		target := fmt.Sprintf("%d%%", *containerResource.Target.AverageUtilization)
		status.PrimaryMetricTarget = &target
	} else if containerResource.Target.AverageValue != nil {
		target := containerResource.Target.AverageValue.String()
		status.PrimaryMetricTarget = &target
	}
}

func (hm *HPAMonitor) handleContainerResourceCurrent(containerResource *autoscalingv2.ContainerResourceMetricStatus, status *HPAStatus) {
	if containerResource.Current.AverageUtilization != nil {
		current := fmt.Sprintf("%d%%", *containerResource.Current.AverageUtilization)
		status.PrimaryMetricCurrent = &current
	} else if containerResource.Current.AverageValue != nil {
		current := containerResource.Current.AverageValue.String()
		status.PrimaryMetricCurrent = &current
	}
}

// External metric handlers
func (hm *HPAMonitor) handleExternalTarget(external *autoscalingv2.ExternalMetricSource, status *HPAStatus) {
	status.PrimaryMetricName = external.Metric.Name
	if external.Target.AverageValue != nil {
		target := hm.normalizeMetricValue(external.Target.AverageValue.String(), external.Metric.Name)
		status.PrimaryMetricTarget = &target
	} else if external.Target.Value != nil {
		target := hm.normalizeMetricValue(external.Target.Value.String(), external.Metric.Name)
		status.PrimaryMetricTarget = &target
	}
}

func (hm *HPAMonitor) handleExternalCurrent(external *autoscalingv2.ExternalMetricStatus, status *HPAStatus) {
	if external.Current.AverageValue != nil {
		current := hm.normalizeMetricValue(external.Current.AverageValue.String(), status.PrimaryMetricName)
		status.PrimaryMetricCurrent = &current
	} else if external.Current.Value != nil {
		current := hm.normalizeMetricValue(external.Current.Value.String(), status.PrimaryMetricName)
		status.PrimaryMetricCurrent = &current
	}
}

// Object metric handlers
func (hm *HPAMonitor) handleObjectTarget(object *autoscalingv2.ObjectMetricSource, status *HPAStatus) {
	status.PrimaryMetricName = object.Metric.Name
	if object.Target.AverageValue != nil {
		target := object.Target.AverageValue.String()
		status.PrimaryMetricTarget = &target
	} else if object.Target.Value != nil {
		target := object.Target.Value.String()
		status.PrimaryMetricTarget = &target
	}
}

func (hm *HPAMonitor) handleObjectCurrent(object *autoscalingv2.ObjectMetricStatus, status *HPAStatus) {
	if object.Current.AverageValue != nil {
		current := object.Current.AverageValue.String()
		status.PrimaryMetricCurrent = &current
	} else if object.Current.Value != nil {
		current := object.Current.Value.String()
		status.PrimaryMetricCurrent = &current
	}
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