package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateData holds all the values to be injected into the templates
type TemplateData struct {
	ACAEnvironmentID         string
	PostgresHost             string
	PostgresUsername         string
	PostgresDatabase         string
	License                  string
	PostgresPassword         string
	PostgresSSLMode          string
	Base64License            string
	Base64PostgresPassword   string
	Base64StorageAccountName string
	Base64StorageAccountKey  string
	ResourceGroup            string
	BindplaneTag             string
	SessionSecret            string
	BindplaneRemoteURL       string
	AzureConnectionString    string
	AzureTopic               string
	AzureSubscriptionID      string
	AzureResourceGroup       string
	AzureNamespace           string
}

// Config holds command line arguments
type Config struct {
	ACAEnvironmentID      string
	PostgresHost          string
	PostgresUsername      string
	PostgresDatabase      string
	License               string
	PostgresPassword      string
	PostgresSSLMode       string
	StorageAccountName    string
	StorageAccountKey     string
	ResourceGroup         string
	OutputDir             string
	TemplatesDir          string
	BindplaneTag          string
	SessionSecret         string
	BindplaneRemoteURL    string
	AzureConnectionString string
	AzureTopic            string
	AzureSubscriptionID   string
	AzureResourceGroup    string
	AzureNamespace        string
}

func main() {
	config := parseFlags()

	if err := validateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	templateData := &TemplateData{
		ACAEnvironmentID:         config.ACAEnvironmentID,
		PostgresHost:             config.PostgresHost,
		PostgresUsername:         config.PostgresUsername,
		PostgresDatabase:         config.PostgresDatabase,
		License:                  config.License,
		PostgresPassword:         config.PostgresPassword,
		PostgresSSLMode:          config.PostgresSSLMode,
		Base64License:            base64.StdEncoding.EncodeToString([]byte(config.License)),
		Base64PostgresPassword:   base64.StdEncoding.EncodeToString([]byte(config.PostgresPassword)),
		Base64StorageAccountName: base64.StdEncoding.EncodeToString([]byte(config.StorageAccountName)),
		Base64StorageAccountKey:  base64.StdEncoding.EncodeToString([]byte(config.StorageAccountKey)),
		ResourceGroup:            config.ResourceGroup,
		BindplaneTag:             config.BindplaneTag,
		SessionSecret:            config.SessionSecret,
		BindplaneRemoteURL:       config.BindplaneRemoteURL,
		AzureConnectionString:    config.AzureConnectionString,
		AzureTopic:               config.AzureTopic,
		AzureSubscriptionID:      config.AzureSubscriptionID,
		AzureResourceGroup:       config.AzureResourceGroup,
		AzureNamespace:           config.AzureNamespace,
	}

	if err := processTemplates(config, templateData); err != nil {
		fmt.Fprintf(os.Stderr, "Error processing templates: %v\n", err)
		os.Exit(1)
	}

	generateDeploymentCommands(config)

	fmt.Printf("Templates processed successfully. Output files generated in: %s\n", config.OutputDir)
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.ACAEnvironmentID, "aca-environment-id", "", "Azure Container Apps Environment ID (required)")
	flag.StringVar(&config.PostgresHost, "postgres-host", "", "PostgreSQL hostname (required)")
	flag.StringVar(&config.PostgresUsername, "postgres-username", "", "PostgreSQL username (required)")
	flag.StringVar(&config.PostgresDatabase, "postgres-database", "", "PostgreSQL database name (required)")
	flag.StringVar(&config.License, "license", "", "Bindplane license key (required)")
	flag.StringVar(&config.PostgresPassword, "postgres-password", "", "PostgreSQL password (required)")
	flag.StringVar(&config.PostgresSSLMode, "postgres-ssl-mode", "disable", "PostgreSQL SSL mode (disable, require, verify-ca, verify-full)")
	flag.StringVar(&config.StorageAccountName, "storage-account-name", "", "Azure Storage Account name (required)")
	flag.StringVar(&config.StorageAccountKey, "storage-account-key", "", "Azure Storage Account key (required)")
	flag.StringVar(&config.ResourceGroup, "resource-group", "", "Azure Resource Group name (required)")
	flag.StringVar(&config.OutputDir, "output-dir", "out", "Output directory for generated files")
	flag.StringVar(&config.TemplatesDir, "templates-dir", "templates", "Templates directory")
	flag.StringVar(&config.BindplaneTag, "bindplane-tag", "1.94.3", "Bindplane image tag (default 1.94.3)")
	flag.StringVar(&config.SessionSecret, "session-secret", "", "Bindplane session secret (required)")
	flag.StringVar(&config.BindplaneRemoteURL, "bindplane-remote-url", "http://localhost:3001", "Bindplane remote URL (default http://localhost:3001)")
	flag.StringVar(&config.AzureConnectionString, "azure-connection-string", "", "Azure Service Bus connection string (required)")
	flag.StringVar(&config.AzureTopic, "azure-topic", "", "Azure Service Bus topic name (required)")
	flag.StringVar(&config.AzureSubscriptionID, "azure-subscription-id", "", "Azure subscription ID (required)")
	flag.StringVar(&config.AzureResourceGroup, "azure-resource-group", "", "Azure resource group name (required)")
	flag.StringVar(&config.AzureNamespace, "azure-namespace", "", "Azure Service Bus namespace (required)")

	flag.Parse()

	return config
}

func validateConfig(config *Config) error {
	required := map[string]string{
		"aca-environment-id":      config.ACAEnvironmentID,
		"postgres-host":           config.PostgresHost,
		"postgres-username":       config.PostgresUsername,
		"postgres-database":       config.PostgresDatabase,
		"license":                 config.License,
		"postgres-password":       config.PostgresPassword,
		"storage-account-name":    config.StorageAccountName,
		"storage-account-key":     config.StorageAccountKey,
		"resource-group":          config.ResourceGroup,
		"session-secret":          config.SessionSecret,
		"azure-connection-string": config.AzureConnectionString,
		"azure-topic":             config.AzureTopic,
		"azure-subscription-id":   config.AzureSubscriptionID,
		"azure-resource-group":    config.AzureResourceGroup,
		"azure-namespace":         config.AzureNamespace,
	}

	var missing []string
	for flag, value := range required {
		if value == "" {
			missing = append(missing, flag)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required flags: %s", strings.Join(missing, ", "))
	}

	return nil
}

func processTemplates(config *Config, data *TemplateData) error {
	// Create output directory
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Template files to process
	templateFiles := []string{
		"bindplane.yaml",
		"jobs.yaml",
		"prometheus.yaml",
		"transform-agent.yaml",
	}

	for _, filename := range templateFiles {
		if err := processTemplate(config, data, filename); err != nil {
			return fmt.Errorf("failed to process template %s: %w", filename, err)
		}
	}

	return nil
}

func processTemplate(config *Config, data *TemplateData, filename string) error {
	templatePath := filepath.Join(config.TemplatesDir, filename)
	outputPath := filepath.Join(config.OutputDir, filename)

	// Read template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	// Parse and execute template
	tmpl, err := template.New(filename).Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", filename, err)
	}

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	// Execute template
	if err := tmpl.Execute(outputFile, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", filename, err)
	}

	fmt.Printf("Generated: %s\n", outputPath)

	return nil
}

func generateDeploymentCommands(config *Config) {
	commandsFile := filepath.Join(config.OutputDir, "deploy.sh")

	commands := []string{
		"#!/bin/bash",
		"# Generated deployment commands for Bindplane Azure Container Apps",
		"",
		"set -e",
		"",
		"echo \"Deploying Bindplane to Azure Container Apps...\"",
		"",
		"# Ensure environment storage exists for Azure Files volumes",
		fmt.Sprintf("ENV_NAME=$(basename %s)", config.ACAEnvironmentID),
		fmt.Sprintf("echo \"Using Container Apps environment: $ENV_NAME in resource group %s\"", config.ResourceGroup),
		fmt.Sprintf("az containerapp env storage set --name \"$ENV_NAME\" --resource-group %s --storage-name prometheus-pv --azure-file-account-name \"%s\" --azure-file-account-key \"%s\" --azure-file-share-name prometheus-data --access-mode ReadWrite || true", config.ResourceGroup, config.StorageAccountName, config.StorageAccountKey),
		"",
		"# Deploy in order to ensure proper dependencies",
		"",
		"echo \"1. Deploying Transform Agent...\"",
		fmt.Sprintf("az containerapp create --name bindplane-transform-agent --resource-group %s --yaml %s/transform-agent.yaml", config.ResourceGroup, config.OutputDir),
		"",
		"echo \"2. Deploying Prometheus...\"",
		fmt.Sprintf("az containerapp create --name bindplane-prometheus --resource-group %s --yaml %s/prometheus.yaml", config.ResourceGroup, config.OutputDir),
		"",
		"echo \"3. Deploying Jobs component...\"",
		fmt.Sprintf("az containerapp create --name bindplane-jobs --resource-group %s --yaml %s/jobs.yaml", config.ResourceGroup, config.OutputDir),
		"",
		"echo \"4. Deploying main Bindplane application...\"",
		fmt.Sprintf("az containerapp create --name bindplane --resource-group %s --yaml %s/bindplane.yaml", config.ResourceGroup, config.OutputDir),
		"",
		"echo \"Deployment complete!\"",
		"",
		"echo \"Checking deployment status...\"",
		fmt.Sprintf("az containerapp list --resource-group %s --query \"[].{Name:name,Status:properties.provisioningState}\" --output table", config.ResourceGroup),
	}

	file, err := os.Create(commandsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create deployment script: %v\n", err)
		return
	}
	defer file.Close()

	for _, cmd := range commands {
		fmt.Fprintln(file, cmd)
	}

	// Make script executable
	if err := os.Chmod(commandsFile, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to make deployment script executable: %v\n", err)
	}

	fmt.Printf("Deployment script generated: %s\n", commandsFile)
}
