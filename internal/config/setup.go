package config

import (
	"os/exec"
	"path/filepath"
)

const (
	ConfigDir = "/etc/wireguard"
	
	ProdTemplate    = "julo-prod-template.conf"
	NonProdTemplate = "julo-nonprod-template.conf"
	ProdConfig      = "julo-prod.conf"
	NonProdConfig   = "julo-nonprod.conf"
	
	ProdEndpoint    = "34.101.166.184:51820"
	NonProdEndpoint = "34.128.85.147:51820"
)

type SetupStatus struct {
	NeedsSetup       bool
	HasTemplates     bool
	HasProdConfig    bool
	HasNonProdConfig bool
	MissingFiles     []string
}

func CheckSetupStatus() (*SetupStatus, error) {
	status := &SetupStatus{
		MissingFiles: []string{},
	}
	
	// Try to check files with sudo to handle permission issues
	return checkSetupStatusWithSudo(status)
}

func checkSetupStatusWithSudo(status *SetupStatus) (*SetupStatus, error) {
	// Check for template files using sudo ls
	filesToCheck := []string{
		ProdTemplate,
		NonProdTemplate,
		ProdConfig,
		NonProdConfig,
	}
	
	// Use sudo ls to check if files exist in /etc/wireguard/
	for _, filename := range filesToCheck {
		filepath := filepath.Join(ConfigDir, filename)
		
		// Use sudo test to check if file exists
		cmd := exec.Command("sudo", "test", "-f", filepath)
		if err := cmd.Run(); err != nil {
			status.MissingFiles = append(status.MissingFiles, filename)
		} else {
			// File exists
			switch filename {
			case ProdTemplate:
				status.HasTemplates = true
			case NonProdTemplate:
				if status.HasTemplates {
					status.HasTemplates = true
				} else {
					status.HasTemplates = true
				}
			case ProdConfig:
				status.HasProdConfig = true
			case NonProdConfig:
				status.HasNonProdConfig = true
			}
		}
	}
	
	// Fix template status - we need both templates to exist
	hasProdTemplate := true
	hasNonprodTemplate := true
	
	for _, missing := range status.MissingFiles {
		if missing == ProdTemplate {
			hasProdTemplate = false
		}
		if missing == NonProdTemplate {
			hasNonprodTemplate = false
		}
	}
	status.HasTemplates = hasProdTemplate && hasNonprodTemplate
	
	// Determine if setup is needed
	// Setup is needed if we don't have templates OR if we don't have at least one working config
	status.NeedsSetup = !status.HasTemplates || (!status.HasProdConfig && !status.HasNonProdConfig)
	
	return status, nil
}