package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// TemplateData holds all the values to be injected into the templates
type TemplateData struct {
	ACAEnvironmentID         string
	PostgresHost             string
	PostgresUsername         string
	PostgresDatabase         string
	License                  string
	PostgresPassword         string
	Base64License            string
	Base64PostgresPassword   string
	Base64StorageAccountName string
	Base64StorageAccountKey  string
	ResourceGroup            string
}

// Config holds command line arguments
type Config struct {
	ACAEnvironmentID   string
	PostgresHost       string
	PostgresUsername   string
	PostgresDatabase   string
	License            string
	PostgresPassword   string
	StorageAccountName string
	StorageAccountKey  string
	ResourceGroup      string
	OutputDir          string
	TemplatesDir       string
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
		Base64License:            base64.StdEncoding.EncodeToString([]byte(config.License)),
		Base64PostgresPassword:   base64.StdEncoding.EncodeToString([]byte(config.PostgresPassword)),
		Base64StorageAccountName: base64.StdEncoding.EncodeToString([]byte(config.StorageAccountName)),
		Base64StorageAccountKey:  base64.StdEncoding.EncodeToString([]byte(config.StorageAccountKey)),
		ResourceGroup:            config.ResourceGroup,
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
	flag.StringVar(&config.StorageAccountName, "storage-account-name", "", "Azure Storage Account name (required)")
	flag.StringVar(&config.StorageAccountKey, "storage-account-key", "", "Azure Storage Account key (required)")
	flag.StringVar(&config.ResourceGroup, "resource-group", "", "Azure Resource Group name (required)")
	flag.StringVar(&config.OutputDir, "output-dir", "out", "Output directory for generated files")
	flag.StringVar(&config.TemplatesDir, "templates-dir", "templates", "Templates directory")

	flag.Parse()

	return config
}

func validateConfig(config *Config) error {
	required := map[string]string{
		"aca-environment-id":   config.ACAEnvironmentID,
		"postgres-host":        config.PostgresHost,
		"postgres-username":    config.PostgresUsername,
		"postgres-database":    config.PostgresDatabase,
		"license":              config.License,
		"postgres-password":    config.PostgresPassword,
		"storage-account-name": config.StorageAccountName,
		"storage-account-key":  config.StorageAccountKey,
		"resource-group":       config.ResourceGroup,
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
		"jobs-migrate.yaml",
		"nats.yaml",
		"prometheus.yaml",
		"transform-agent.yaml",
	}

	for i, filename := range templateFiles {
		if err := processTemplate(config, data, filename); err != nil {
			return fmt.Errorf("failed to process template %s: %w", filename, err)
		}

		// Add delay between template generations (except after the last one)
		if i < len(templateFiles)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

func processTemplate(config *Config, data *TemplateData, filename string) error {
	templatePath := filepath.Join(config.TemplatesDir, filename)
	outputPath := filepath.Join(config.OutputDir, filename)

	fmt.Printf("Processing template: %s -> %s\n", templatePath, outputPath)

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
		fmt.Sprintf("az containerapp env storage set --name $ENV_NAME --resource-group %s --storage-name prometheus-pv --azure-file-account-name %s --azure-file-account-key %s --azure-file-share-name prometheus-data --access-mode ReadWrite || true", config.ResourceGroup, config.StorageAccountName, config.StorageAccountKey),
		fmt.Sprintf("az containerapp env storage set --name $ENV_NAME --resource-group %s --storage-name prometheus-config-pv --azure-file-account-name %s --azure-file-account-key %s --azure-file-share-name prometheus-config --access-mode ReadWrite || true", config.ResourceGroup, config.StorageAccountName, config.StorageAccountKey),
		"",
		"# Helper to wait until a Container App is ready",
		"wait_for_app() {",
		"  local name=\"$1\"",
		fmt.Sprintf("  echo \"Waiting for %s/$name to become ready...\"", config.ResourceGroup),
		fmt.Sprintf("  for i in {1..60}; do ready=$(az containerapp show --name \"$name\" --resource-group %s --query \"properties.latestReadyRevisionName!=null && properties.latestReadyRevisionName==properties.latestRevisionName\" -o tsv 2>/dev/null || echo false); if [ \"$ready\" = \"true\" ]; then echo \"$name is ready.\"; return 0; fi; sleep 5; done", config.ResourceGroup),
		"  echo \"Timed out waiting for $name to become ready\"",
		"  return 1",
		"}",
		"",
		"# Deploy in order to ensure proper dependencies",
		"",
		"echo \"1. Deploying NATS cluster...\"",
		fmt.Sprintf("az containerapp create --name bindplane-nats --resource-group %s --yaml %s/nats.yaml", config.ResourceGroup, config.OutputDir),
		"wait_for_app bindplane-nats",
		"",
		"echo \"2. Deploying Prometheus...\"",
		fmt.Sprintf("az containerapp create --name bindplane-prometheus --resource-group %s --yaml %s/prometheus.yaml", config.ResourceGroup, config.OutputDir),
		"wait_for_app bindplane-prometheus",
		"",
		"echo \"3. Deploying Transform Agent...\"",
		fmt.Sprintf("az containerapp create --name bindplane-transform-agent --resource-group %s --yaml %s/transform-agent.yaml", config.ResourceGroup, config.OutputDir),
		"wait_for_app bindplane-transform-agent",
		"",
		"echo \"4. Deploying Jobs Migrate component...\"",
		fmt.Sprintf("az containerapp create --name bindplane-jobs-migrate --resource-group %s --yaml %s/jobs-migrate.yaml", config.ResourceGroup, config.OutputDir),
		"wait_for_app bindplane-jobs-migrate",
		"",
		"echo \"5. Deploying Jobs component...\"",
		fmt.Sprintf("az containerapp create --name bindplane-jobs --resource-group %s --yaml %s/jobs.yaml", config.ResourceGroup, config.OutputDir),
		"wait_for_app bindplane-jobs",
		"",
		"echo \"6. Deploying main Bindplane application...\"",
		fmt.Sprintf("az containerapp create --name bindplane --resource-group %s --yaml %s/bindplane.yaml", config.ResourceGroup, config.OutputDir),
		"wait_for_app bindplane",
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
