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
		"secrets.yaml",
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
		"# Deploy in order to ensure proper dependencies",
		"",
		"echo \"1. Deploying secrets and storage...\"",
		fmt.Sprintf("az containerapp apply -f %s/secrets.yaml --resource-group %s", config.OutputDir, config.ResourceGroup),
		"",
		"echo \"2. Deploying Prometheus...\"",
		fmt.Sprintf("az containerapp apply -f %s/prometheus.yaml --resource-group %s", config.OutputDir, config.ResourceGroup),
		"",
		"echo \"3. Deploying Transform Agent...\"",
		fmt.Sprintf("az containerapp apply -f %s/transform-agent.yaml --resource-group %s", config.OutputDir, config.ResourceGroup),
		"",
		"echo \"4. Deploying NATS cluster...\"",
		fmt.Sprintf("az containerapp apply -f %s/nats.yaml --resource-group %s", config.OutputDir, config.ResourceGroup),
		"",
		"echo \"5. Deploying Jobs Migrate component...\"",
		fmt.Sprintf("az containerapp apply -f %s/jobs-migrate.yaml --resource-group %s", config.OutputDir, config.ResourceGroup),
		"",
		"echo \"6. Deploying Jobs component...\"",
		fmt.Sprintf("az containerapp apply -f %s/jobs.yaml --resource-group %s", config.OutputDir, config.ResourceGroup),
		"",
		"echo \"7. Deploying main Bindplane application...\"",
		fmt.Sprintf("az containerapp apply -f %s/bindplane.yaml --resource-group %s", config.OutputDir, config.ResourceGroup),
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
