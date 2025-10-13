package consensus

import (
	"math"

	"github.com/avinashshinde/agentmesh-cortex/pkg/types"
)

// GenerateWaggleDance creates a waggle dance based on proposal content
// The waggle dance encodes the "quality" and "enthusiasm" of the proposal
func GenerateWaggleDance(content map[string]any) types.WaggleDance {
	// Calculate intensity based on proposal properties
	intensity := calculateIntensity(content)

	// Duration correlates with intensity (higher intensity = longer dance)
	duration := int(intensity * 1000) // 0-1000ms

	// Angle represents direction/quality (0-360 degrees)
	angle := calculateAngle(content)

	// Repetitions based on intensity (more repetitions = stronger signal)
	repetitions := int(intensity * 10) // 0-10 repetitions

	return types.WaggleDance{
		Intensity:   intensity,
		Duration:    duration,
		Angle:       angle,
		Repetitions: max(1, repetitions), // At least 1 repetition
	}
}

// calculateIntensity computes the intensity of the waggle dance
// In nature, bees waggle more vigorously for better food sources
func calculateIntensity(content map[string]any) float64 {
	intensity := 0.5 // Default medium intensity

	// Check for priority indicators
	if priority, ok := content["priority"].(string); ok {
		switch priority {
		case "high", "critical":
			intensity = 0.9
		case "medium":
			intensity = 0.6
		case "low":
			intensity = 0.3
		}
	}

	// Check for urgency
	if urgent, ok := content["urgent"].(bool); ok && urgent {
		intensity = min(1.0, intensity+0.2)
	}

	// Check for confidence score
	if confidence, ok := content["confidence"].(float64); ok {
		intensity = (intensity + confidence) / 2.0
	}

	// Check for amount (for approval scenarios)
	if amount, ok := content["amount"].(float64); ok {
		// Higher amounts get higher intensity (capped)
		amountIntensity := math.Log10(amount+1) / 10.0
		intensity = (intensity + min(1.0, amountIntensity)) / 2.0
	}

	return clamp(intensity, 0.0, 1.0)
}

// calculateAngle computes the angle of the waggle dance
// In nature, bees use angle to indicate direction to food source
// Here, we use it to encode proposal type and quality metrics
func calculateAngle(content map[string]any) float64 {
	baseAngle := 180.0 // Default middle angle

	// Encode proposal type in angle
	if proposalType, ok := content["type"].(string); ok {
		switch proposalType {
		case "approval":
			baseAngle = 90.0 // Right angle for approvals
		case "rejection":
			baseAngle = 270.0 // Left angle for rejections
		case "action":
			baseAngle = 180.0 // Straight for actions
		case "topology":
			baseAngle = 0.0 // Up for topology changes
		}
	}

	// Add variation based on quality
	if quality, ok := content["quality"].(float64); ok {
		// Quality adds +/- 45 degrees variation
		baseAngle += (quality - 0.5) * 90.0
	}

	// Normalize to 0-360 range
	for baseAngle < 0 {
		baseAngle += 360.0
	}
	for baseAngle >= 360.0 {
		baseAngle -= 360.0
	}

	return baseAngle
}

// InterpretWaggleDance interprets a waggle dance to extract meaning
// This is used by agents to understand proposal quality
func InterpretWaggleDance(waggle types.WaggleDance) map[string]interface{} {
	interpretation := make(map[string]interface{})

	// Interpret intensity
	if waggle.Intensity > 0.8 {
		interpretation["quality"] = "high"
		interpretation["enthusiasm"] = "very_enthusiastic"
	} else if waggle.Intensity > 0.5 {
		interpretation["quality"] = "medium"
		interpretation["enthusiasm"] = "moderate"
	} else {
		interpretation["quality"] = "low"
		interpretation["enthusiasm"] = "lukewarm"
	}

	// Interpret angle to determine proposal direction
	if waggle.Angle < 45 || waggle.Angle >= 315 {
		interpretation["direction"] = "forward"
	} else if waggle.Angle >= 45 && waggle.Angle < 135 {
		interpretation["direction"] = "right"
	} else if waggle.Angle >= 135 && waggle.Angle < 225 {
		interpretation["direction"] = "backward"
	} else {
		interpretation["direction"] = "left"
	}

	// Interpret repetitions
	interpretation["persistence"] = waggle.Repetitions

	// Interpret duration
	interpretation["duration_category"] = "medium"
	if waggle.Duration > 750 {
		interpretation["duration_category"] = "long"
	} else if waggle.Duration < 250 {
		interpretation["duration_category"] = "short"
	}

	return interpretation
}

// CompareWaggleDances compares two waggle dances and returns the stronger one
// Used for cross-inhibition: competing proposals suppress weaker ones
func CompareWaggleDances(w1, w2 types.WaggleDance) int {
	score1 := w1.Intensity * float64(w1.Repetitions)
	score2 := w2.Intensity * float64(w2.Repetitions)

	if score1 > score2 {
		return 1
	} else if score1 < score2 {
		return -1
	}
	return 0
}

// CalculateCrossInhibition calculates how much one proposal inhibits another
// In bee colonies, stronger signals suppress weaker competing signals
func CalculateCrossInhibition(dominant, weaker types.WaggleDance) float64 {
	intensityDiff := dominant.Intensity - weaker.Intensity
	repetitionDiff := float64(dominant.Repetitions-weaker.Repetitions) / 10.0

	// Combined inhibition effect
	inhibition := (intensityDiff + repetitionDiff) / 2.0

	return clamp(inhibition, 0.0, 1.0)
}

// Helper functions
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
