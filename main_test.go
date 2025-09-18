package main

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestTemplateProcessing(t *testing.T) {
	// Test data
	testData := &TemplateData{
		ACAEnvironmentID:         "test-env-12345",
		PostgresHost:             "test-postgres.postgres.database.azure.com",
		PostgresUsername:         "test_user",
		PostgresDatabase:         "test_db",
		License:                  "test-license-key",
		PostgresPassword:         "test-password",
		Base64License:            base64.StdEncoding.EncodeToString([]byte("test-license-key")),
		Base64PostgresPassword:   base64.StdEncoding.EncodeToString([]byte("test-password")),
		Base64StorageAccountName: base64.StdEncoding.EncodeToString([]byte("teststorageaccount")),
		Base64StorageAccountKey:  base64.StdEncoding.EncodeToString([]byte("test-storage-key")),
		ResourceGroup:            "test-rg",
		BindplaneTag:             "1.94.3",
		SessionSecret:            "test-session-secret",
		BindplaneRemoteURL:       "http://localhost:3001",
	}

	templateFiles := []string{
		"bindplane.yaml",
		"jobs.yaml",
		"nats-0.yaml",
		"nats-1.yaml",
		"nats-2.yaml",
		"prometheus.yaml",
		"transform-agent.yaml",
	}

	for _, filename := range templateFiles {
		t.Run(filename, func(t *testing.T) {
			// Read template
			templatePath := filepath.Join("templates", filename)
			templateContent, err := os.ReadFile(templatePath)
			if err != nil {
				t.Fatalf("Failed to read template %s: %v", templatePath, err)
			}

			// Parse and execute template
			tmpl, err := template.New(filename).Parse(string(templateContent))
			if err != nil {
				t.Fatalf("Failed to parse template %s: %v", filename, err)
			}

			var output bytes.Buffer
			if err := tmpl.Execute(&output, testData); err != nil {
				t.Fatalf("Failed to execute template %s: %v", filename, err)
			}

			// Read golden file
			goldenPath := filepath.Join("testdata", filename)
			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				// If golden file doesn't exist, create it
				if os.IsNotExist(err) {
					if err := os.WriteFile(goldenPath, output.Bytes(), 0644); err != nil {
						t.Fatalf("Failed to write golden file %s: %v", goldenPath, err)
					}
					t.Logf("Created golden file %s", goldenPath)
					return
				}
				t.Fatalf("Failed to read golden file %s: %v", goldenPath, err)
			}

			// Compare output with golden file
			if !bytes.Equal(output.Bytes(), expected) {
				t.Errorf("Output for %s doesn't match golden file.\nExpected:\n%s\nGot:\n%s",
					filename, string(expected), output.String())
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				ACAEnvironmentID:   "test-env",
				PostgresHost:       "test-host",
				PostgresUsername:   "test-user",
				PostgresDatabase:   "test-db",
				License:            "test-license",
				PostgresPassword:   "test-pass",
				StorageAccountName: "test-storage",
				StorageAccountKey:  "test-key",
				ResourceGroup:      "test-rg",
				SessionSecret:      "test-session-secret",
			},
			wantError: false,
		},
		{
			name: "missing aca-environment-id",
			config: &Config{
				PostgresHost:       "test-host",
				PostgresUsername:   "test-user",
				PostgresDatabase:   "test-db",
				License:            "test-license",
				PostgresPassword:   "test-pass",
				StorageAccountName: "test-storage",
				StorageAccountKey:  "test-key",
				ResourceGroup:      "test-rg",
			},
			wantError: true,
			errorMsg:  "aca-environment-id",
		},
		{
			name: "missing postgres-host",
			config: &Config{
				ACAEnvironmentID:   "test-env",
				PostgresUsername:   "test-user",
				PostgresDatabase:   "test-db",
				License:            "test-license",
				PostgresPassword:   "test-pass",
				StorageAccountName: "test-storage",
				StorageAccountKey:  "test-key",
				ResourceGroup:      "test-rg",
			},
			wantError: true,
			errorMsg:  "postgres-host",
		},
		{
			name: "missing multiple fields",
			config: &Config{
				ACAEnvironmentID: "test-env",
				PostgresHost:     "test-host",
			},
			wantError: true,
			errorMsg:  "postgres-username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestBase64Encoding(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"test-license", base64.StdEncoding.EncodeToString([]byte("test-license"))},
		{"password123", base64.StdEncoding.EncodeToString([]byte("password123"))},
		{"storageaccount", base64.StdEncoding.EncodeToString([]byte("storageaccount"))},
		{"storage-key-123", base64.StdEncoding.EncodeToString([]byte("storage-key-123"))},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := base64.StdEncoding.EncodeToString([]byte(tc.input))
			if result != tc.expected {
				t.Errorf("Base64 encoding failed. Expected: %s, Got: %s", tc.expected, result)
			}
		})
	}
}

func TestTemplateDataCreation(t *testing.T) {
	config := &Config{
		ACAEnvironmentID:   "test-env-12345",
		PostgresHost:       "test-postgres.postgres.database.azure.com",
		PostgresUsername:   "test_user",
		PostgresDatabase:   "test_db",
		License:            "test-license-key",
		PostgresPassword:   "test-password",
		StorageAccountName: "teststorageaccount",
		StorageAccountKey:  "test-storage-key",
		ResourceGroup:      "test-rg",
		BindplaneTag:       "1.94.3",
		SessionSecret:      "test-session-secret",
		BindplaneRemoteURL: "http://localhost:3001",
	}

	data := &TemplateData{
		ACAEnvironmentID:         config.ACAEnvironmentID,
		PostgresHost:             config.PostgresHost,
		PostgresUsername:         config.PostgresUsername,
		PostgresDatabase:         config.PostgresDatabase,
		Base64License:            base64.StdEncoding.EncodeToString([]byte(config.License)),
		Base64PostgresPassword:   base64.StdEncoding.EncodeToString([]byte(config.PostgresPassword)),
		Base64StorageAccountName: base64.StdEncoding.EncodeToString([]byte(config.StorageAccountName)),
		Base64StorageAccountKey:  base64.StdEncoding.EncodeToString([]byte(config.StorageAccountKey)),
		ResourceGroup:            config.ResourceGroup,
		BindplaneRemoteURL:       "http://localhost:3001",
	}

	// Verify plain text fields
	if data.ACAEnvironmentID != config.ACAEnvironmentID {
		t.Errorf("ACAEnvironmentID mismatch. Expected: %s, Got: %s", config.ACAEnvironmentID, data.ACAEnvironmentID)
	}

	if data.PostgresHost != config.PostgresHost {
		t.Errorf("PostgresHost mismatch. Expected: %s, Got: %s", config.PostgresHost, data.PostgresHost)
	}

	// Verify base64 encoded fields
	expectedLicense := base64.StdEncoding.EncodeToString([]byte(config.License))
	if data.Base64License != expectedLicense {
		t.Errorf("Base64License mismatch. Expected: %s, Got: %s", expectedLicense, data.Base64License)
	}

	expectedPassword := base64.StdEncoding.EncodeToString([]byte(config.PostgresPassword))
	if data.Base64PostgresPassword != expectedPassword {
		t.Errorf("Base64PostgresPassword mismatch. Expected: %s, Got: %s", expectedPassword, data.Base64PostgresPassword)
	}
}

func TestTemplateExecution(t *testing.T) {
	// Test a simple template execution
	templateStr := `
environmentId: {{.ACAEnvironmentID}}
postgres:
  host: {{.PostgresHost}}
  username: {{.PostgresUsername}}
  database: {{.PostgresDatabase}}
secrets:
  license: {{.Base64License}}
  password: {{.Base64PostgresPassword}}
`

	data := &TemplateData{
		ACAEnvironmentID:       "test-env",
		PostgresHost:           "test-host",
		PostgresUsername:       "test-user",
		PostgresDatabase:       "test-db",
		Base64License:          "dGVzdC1saWNlbnNl",     // base64 of "test-license"
		Base64PostgresPassword: "dGVzdC1wYXNzd29yZA==", // base64 of "test-password"
	}

	tmpl, err := template.New("test").Parse(templateStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	result := output.String()

	// Check if template values were properly substituted
	expectedSubstitutions := []string{
		"environmentId: test-env",
		"host: test-host",
		"username: test-user",
		"database: test-db",
		"license: dGVzdC1saWNlbnNl",
		"password: dGVzdC1wYXNzd29yZA==",
	}

	for _, expected := range expectedSubstitutions {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected template output to contain %q, but it didn't. Output:\n%s", expected, result)
		}
	}
}
