package config

// ResolveModel takes a role name ("large", "vision", etc.) and returns
// the actual model name string.
// Fallback chain: vision → large, ocr → vision → large,
//                 small → medium → large, medium → large
func (c *Config) ResolveModel(role string) string {
	switch role {
	case "large":
		if c.Models.Large != "" {
			return c.Models.Large
		}
		return DefaultLargeModel

	case "medium":
		if c.Models.Medium != "" {
			return c.Models.Medium
		}
		// Fall back to large
		if c.Models.Large != "" {
			return c.Models.Large
		}
		return DefaultLargeModel

	case "small":
		if c.Models.Small != "" {
			return c.Models.Small
		}
		// Fall back to medium
		if c.Models.Medium != "" {
			return c.Models.Medium
		}
		// Fall back to large
		if c.Models.Large != "" {
			return c.Models.Large
		}
		return DefaultLargeModel

	case "vision":
		if c.Models.Vision != "" {
			return c.Models.Vision
		}
		// Fall back to large
		if c.Models.Large != "" {
			return c.Models.Large
		}
		return DefaultLargeModel

	case "vision_small":
		if c.Models.VisionSmall != "" {
			return c.Models.VisionSmall
		}
		// Fall back to vision
		if c.Models.Vision != "" {
			return c.Models.Vision
		}
		// Fall back to large
		if c.Models.Large != "" {
			return c.Models.Large
		}
		return DefaultLargeModel

	case "ocr":
		if c.Models.OCR != "" {
			return c.Models.OCR
		}
		// Fall back to vision
		if c.Models.Vision != "" {
			return c.Models.Vision
		}
		// Fall back to large
		if c.Models.Large != "" {
			return c.Models.Large
		}
		return DefaultLargeModel

	default:
		// Unknown role, treat it as a model name directly
		if role != "" {
			return role
		}
		return DefaultLargeModel
	}
}

// ResolveBackend takes a model name and returns the BackendConfig to use.
// If the model has a specific backend in config.Backends, use that.
// Otherwise use config.API (the default).
func (c *Config) ResolveBackend(modelName string) BackendConfig {
	if modelName == "" {
		return c.API.toBackendConfig()
	}

	// Check if model has a specific backend config
	if backend, ok := c.Backends[modelName]; ok {
		return backend
	}

	// Default to API config
	return c.API.toBackendConfig()
}

// ResolveAPIType returns "openai" or "anthropic" for a given model.
func (c *Config) ResolveAPIType(modelName string) string {
	// Check model-specific backend first
	if backend, ok := c.Backends[modelName]; ok {
		if backend.APIType != "" {
			return backend.APIType
		}
	}

	// Fall back to API config
	if c.API.APIType != "" {
		return c.API.APIType
	}

	// Default to openai
	return "openai"
}

// toBackendConfig converts APIConfig to BackendConfig
func (api *APIConfig) toBackendConfig() BackendConfig {
	return BackendConfig{
		BaseURL:  api.BaseURL,
		APIKey:   api.APIKey,
		APIType:  api.APIType,
		Timeout:  api.Timeout,
	}
}
