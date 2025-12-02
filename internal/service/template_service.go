package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

// TemplateService handles template rendering and validation
type TemplateService interface {
	Render(template string, customer *models.Customer) (string, error)
	ValidateTemplate(template string) error
	ExtractPlaceholders(template string) []string
}

type templateService struct {
	placeholderPattern *regexp.Regexp
}

// NewTemplateService creates a new template service
func NewTemplateService() TemplateService {
	return &templateService{
		placeholderPattern: regexp.MustCompile(`\{([a-z_]+)\}`),
	}
}

// Render replaces placeholders in template with customer data
// Missing fields are replaced with empty strings
func (s *templateService) Render(template string, customer *models.Customer) (string, error) {
	if customer == nil {
		return "", models.ErrInvalidInput("customer cannot be nil")
	}

	// Map customer fields to their values
	fieldMap := map[string]string{
		"first_name":        customer.FirstName,
		"last_name":         customer.LastName,
		"location":          customer.Location,
		"preferred_product": customer.PreferredProduct,
		"phone":             customer.Phone,
	}

	// Replace all placeholders
	result := s.placeholderPattern.ReplaceAllStringFunc(template, func(match string) string {
		// Extract field name from {field_name}
		fieldName := strings.Trim(match, "{}")

		// Return field value or empty string if not found
		if value, exists := fieldMap[fieldName]; exists {
			return value
		}

		// Unknown placeholder - return empty string
		return ""
	})

	return result, nil
}

// ValidateTemplate checks if template syntax is valid
func (s *templateService) ValidateTemplate(template string) error {
	if template == "" {
		return models.ErrInvalidInput("template cannot be empty")
	}

	// Extract all placeholders
	placeholders := s.ExtractPlaceholders(template)

	// Define valid placeholders
	validPlaceholders := map[string]bool{
		"first_name":        true,
		"last_name":         true,
		"location":          true,
		"preferred_product": true,
		"phone":             true,
	}

	// Check for invalid placeholders
	var invalidPlaceholders []string
	for _, placeholder := range placeholders {
		if !validPlaceholders[placeholder] {
			invalidPlaceholders = append(invalidPlaceholders, placeholder)
		}
	}

	if len(invalidPlaceholders) > 0 {
		return models.ErrInvalidInput(
			fmt.Sprintf("invalid placeholders: %s. Valid placeholders are: first_name, last_name, location, preferred_product, phone",
				strings.Join(invalidPlaceholders, ", ")),
		)
	}

	return nil
}

// ExtractPlaceholders returns all placeholders found in template
func (s *templateService) ExtractPlaceholders(template string) []string {
	matches := s.placeholderPattern.FindAllStringSubmatch(template, -1)
	placeholders := make([]string, 0, len(matches))

	for _, match := range matches {
		if len(match) > 1 {
			placeholders = append(placeholders, match[1])
		}
	}

	return placeholders
}
