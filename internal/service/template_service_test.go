package service

import (
	"testing"

	"github.com/Raymond9734/smsleopard-backend-challenge/internal/models"
)

func TestTemplateService_Render(t *testing.T) {
	tests := []struct {
		name     string
		template string
		customer *models.Customer
		want     string
		wantErr  bool
	}{
		{
			name:     "all fields present",
			template: "Hi {first_name}, check out {preferred_product} in {location}!",
			customer: &models.Customer{
				FirstName:        "Alice",
				PreferredProduct: "Running Shoes",
				Location:         "Nairobi",
			},
			want:    "Hi Alice, check out Running Shoes in Nairobi!",
			wantErr: false,
		},
		{
			name:     "missing first_name",
			template: "Hi {first_name}, welcome!",
			customer: &models.Customer{
				FirstName: "",
			},
			want:    "Hi , welcome!",
			wantErr: false,
		},
		{
			name:     "missing preferred_product",
			template: "Check out {preferred_product}!",
			customer: &models.Customer{
				PreferredProduct: "",
			},
			want:    "Check out !",
			wantErr: false,
		},
		{
			name:     "multiple same placeholders",
			template: "Hi {first_name}, yes {first_name}, you!",
			customer: &models.Customer{
				FirstName: "Bob",
			},
			want:    "Hi Bob, yes Bob, you!",
			wantErr: false,
		},
		{
			name:     "all placeholders",
			template: "{first_name} {last_name} from {location} likes {preferred_product}, call {phone}",
			customer: &models.Customer{
				FirstName:        "Alice",
				LastName:         "Mwangi",
				Location:         "Nairobi",
				PreferredProduct: "Shoes",
				Phone:            "+254712345001",
			},
			want:    "Alice Mwangi from Nairobi likes Shoes, call +254712345001",
			wantErr: false,
		},
		{
			name:     "no placeholders",
			template: "This is a plain message",
			customer: &models.Customer{
				FirstName: "Alice",
			},
			want:    "This is a plain message",
			wantErr: false,
		},
		{
			name:     "empty template",
			template: "",
			customer: &models.Customer{
				FirstName: "Alice",
			},
			want:    "",
			wantErr: false,
		},
		{
			name:     "nil customer",
			template: "Hi {first_name}",
			customer: nil,
			want:    "",
			wantErr: true,
		},
		{
			name:     "malformed placeholder (missing closing brace)",
			template: "Hi {first_name",
			customer: &models.Customer{
				FirstName: "Alice",
			},
			want:    "Hi {first_name",
			wantErr: false,
		},
		{
			name:     "placeholder with spaces (should not match)",
			template: "Hi { first_name }",
			customer: &models.Customer{
				FirstName: "Alice",
			},
			want:    "Hi { first_name }",
			wantErr: false,
		},
		{
			name:     "case sensitive placeholders",
			template: "Hi {First_Name}",
			customer: &models.Customer{
				FirstName: "Alice",
			},
			want:    "Hi {First_Name}",
			wantErr: false,
		},
		{
			name:     "special characters in customer data",
			template: "Hi {first_name}!",
			customer: &models.Customer{
				FirstName: "O'Brien",
			},
			want:    "Hi O'Brien!",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTemplateService()
			got, err := svc.Render(tt.template, tt.customer)

			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("Render() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplateService_ExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []string
	}{
		{
			name:     "single placeholder",
			template: "Hi {first_name}",
			want:     []string{"first_name"},
		},
		{
			name:     "multiple placeholders",
			template: "Hi {first_name} from {location}",
			want:     []string{"first_name", "location"},
		},
		{
			name:     "duplicate placeholders",
			template: "Hi {first_name}, yes {first_name}",
			want:     []string{"first_name", "first_name"},
		},
		{
			name:     "no placeholders",
			template: "Plain text message",
			want:     []string{},
		},
		{
			name:     "empty template",
			template: "",
			want:     []string{},
		},
		{
			name:     "malformed placeholders",
			template: "Hi {first_name and {last_name}",
			want:     []string{"last_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTemplateService()
			got := svc.ExtractPlaceholders(tt.template)

			if len(got) != len(tt.want) {
				t.Errorf("ExtractPlaceholders() returned %d placeholders, want %d", len(got), len(tt.want))
				return
			}

			for i, placeholder := range got {
				if placeholder != tt.want[i] {
					t.Errorf("ExtractPlaceholders()[%d] = %v, want %v", i, placeholder, tt.want[i])
				}
			}
		})
	}
}

func BenchmarkTemplateService_Render(b *testing.B) {
	svc := NewTemplateService()
	template := "Hi {first_name} {last_name}, check out {preferred_product} in {location}! Call {phone}"
	customer := &models.Customer{
		FirstName:        "Alice",
		LastName:         "Mwangi",
		Location:         "Nairobi",
		PreferredProduct: "Running Shoes",
		Phone:            "+254712345001",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Render(template, customer)
	}
}
