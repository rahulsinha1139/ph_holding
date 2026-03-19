// ============================================================================
// ASSETS SERVICE - Database-backed binary assets (letterhead, logos, etc.)
//
// PURPOSE: Store assets in database so they sync via Supabase automatically
// No need to include asset folders in deployment packages
// ============================================================================

package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Asset represents a binary asset stored in the database
type Asset struct {
	ID          string    `gorm:"primaryKey;size:50" json:"id"`
	Name        string    `gorm:"size:255;uniqueIndex" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	MimeType    string    `gorm:"size:100" json:"mime_type"`
	Data        string    `gorm:"type:text" json:"data"` // Base64 encoded
	Size        int64     `json:"size"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName ensures the table is named 'assets'
func (Asset) TableName() string {
	return "assets"
}

// AssetInfo is returned when listing assets (without the data)
type AssetInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MimeType    string    `json:"mime_type"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
}

// Known asset names
const (
	AssetLetterhead = "letterhead"
	AssetLogo       = "company_logo"
	AssetStamp      = "company_stamp"
)

// EnsureAssetsTable creates the assets table if it doesn't exist
func (a *App) EnsureAssetsTable() error {
	return a.db.AutoMigrate(&Asset{})
}

// UploadAsset stores a binary file as a base64-encoded asset in the database
func (a *App) UploadAsset(name, description, filePath string) error {
	if err := a.requirePermission("settings:update"); err != nil {
		return err
	}
	// Read file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Determine MIME type from extension
	ext := filepath.Ext(filePath)
	mimeType := getMimeType(ext)

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(data)

	// Create or update asset
	asset := Asset{
		ID:          name, // Use name as ID for easy lookup
		Name:        name,
		Description: description,
		MimeType:    mimeType,
		Data:        encoded,
		Size:        int64(len(data)),
	}

	// Upsert
	result := a.db.Where("name = ?", name).First(&Asset{})
	if result.Error != nil {
		// Create new
		if err := a.db.Create(&asset).Error; err != nil {
			return fmt.Errorf("failed to create asset: %w", err)
		}
		log.Printf("Created asset: %s (%d bytes)", name, len(data))
	} else {
		// Update existing
		if err := a.db.Model(&Asset{}).Where("name = ?", name).Updates(asset).Error; err != nil {
			return fmt.Errorf("failed to update asset: %w", err)
		}
		log.Printf("Updated asset: %s (%d bytes)", name, len(data))
	}

	return nil
}

// getAssetInternal retrieves an asset without permission check (for internal use like PDF generation)
func (a *App) getAssetInternal(name string) ([]byte, error) {
	var asset Asset
	if err := a.db.Where("name = ?", name).First(&asset).Error; err != nil {
		return nil, fmt.Errorf("asset not found: %s", name)
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(asset.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode asset: %w", err)
	}

	return data, nil
}

// GetAsset retrieves an asset from the database and returns the raw bytes (Wails-bound, requires permission)
func (a *App) GetAsset(name string) ([]byte, error) {
	if err := a.requirePermission("settings:view"); err != nil {
		return nil, err
	}
	return a.getAssetInternal(name)
}

// GetAssetToFile retrieves an asset and saves it to a file, returning the path
// This is useful for PDF generation which needs a file path (internal, no permission check)
func (a *App) GetAssetToFile(name string) (string, error) {
	data, err := a.getAssetInternal(name)
	if err != nil {
		return "", err
	}

	// Get cache directory
	cacheDir := a.getAssetCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Get asset info for extension
	var asset Asset
	a.db.Where("name = ?", name).First(&asset)
	ext := getExtensionFromMime(asset.MimeType)

	// Write to cache file
	cachePath := filepath.Join(cacheDir, name+ext)
	if err := ioutil.WriteFile(cachePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write cache file: %w", err)
	}

	return cachePath, nil
}

// ListAssets returns info about all stored assets (without the data)
func (a *App) ListAssets() ([]AssetInfo, error) {
	if err := a.requirePermission("settings:view"); err != nil {
		return nil, err
	}
	var assets []Asset
	if err := a.db.Find(&assets).Error; err != nil {
		return nil, err
	}

	infos := make([]AssetInfo, len(assets))
	for i, asset := range assets {
		infos[i] = AssetInfo{
			ID:          asset.ID,
			Name:        asset.Name,
			Description: asset.Description,
			MimeType:    asset.MimeType,
			Size:        asset.Size,
			CreatedAt:   asset.CreatedAt,
		}
	}
	return infos, nil
}

// DeleteAsset removes an asset from the database
func (a *App) DeleteAsset(name string) error {
	if err := a.requirePermission("settings:update"); err != nil {
		return err
	}
	result := a.db.Where("name = ?", name).Delete(&Asset{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("asset not found: %s", name)
	}

	// Also delete cached file
	cacheDir := a.getAssetCacheDir()
	files, _ := filepath.Glob(filepath.Join(cacheDir, name+".*"))
	for _, f := range files {
		os.Remove(f)
	}

	return nil
}

// HasAsset checks if an asset exists in the database
func (a *App) HasAsset(name string) bool {
	var count int64
	a.db.Model(&Asset{}).Where("name = ?", name).Count(&count)
	return count > 0
}

// InitializeDefaultAssets uploads default assets if they don't exist
// Called during app startup
func (a *App) InitializeDefaultAssets() {
	// Ensure table exists
	a.EnsureAssetsTable()

	// Check if letterhead already exists
	if a.HasAsset(AssetLetterhead) {
		log.Println("Letterhead asset already in database")
		return
	}

	// Try to find and upload the letterhead
	letterheadFile := "PH Trading Quotation Template.png"
	candidatePaths := []string{}

	// Check various locations - GetApplicationPaths may return nil if no license is activated
	paths := a.getAppPaths()
	if paths != nil {
		candidatePaths = append(candidatePaths, filepath.Join(paths.ProjectRoot, "CRITICAL_PH_CONTEXT_SSOT", letterheadFile))
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidatePaths = append(candidatePaths, filepath.Join(exeDir, "..", "..", "CRITICAL_PH_CONTEXT_SSOT", letterheadFile))
		candidatePaths = append(candidatePaths, filepath.Join(exeDir, "CRITICAL_PH_CONTEXT_SSOT", letterheadFile))
	}

	if cwd, err := os.Getwd(); err == nil {
		candidatePaths = append(candidatePaths, filepath.Join(cwd, "CRITICAL_PH_CONTEXT_SSOT", letterheadFile))
	}

	// Find and upload
	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			if err := a.UploadAsset(AssetLetterhead, "PH Trading letterhead template for PDF generation", path); err != nil {
				log.Printf("Warning: Failed to upload letterhead: %v", err)
			} else {
				log.Printf("Letterhead uploaded to database from: %s", path)
			}
			return
		}
	}

	log.Println("Warning: Letterhead file not found in any location")
}

// GetLetterheadPath returns the path to the letterhead image
// First checks database cache, then falls back to file system
func (a *App) GetLetterheadPath() string {
	// Try to get from database first
	if a.HasAsset(AssetLetterhead) {
		cachePath, err := a.GetAssetToFile(AssetLetterhead)
		if err == nil {
			return cachePath
		}
		log.Printf("Warning: Failed to extract letterhead from DB: %v", err)
	}

	// Fall back to file system search
	return a.letterheadImagePath()
}

// getAssetCacheDir returns the directory for cached asset files
func (a *App) getAssetCacheDir() string {
	// Use AppData on Windows, or temp directory
	if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
		return filepath.Join(appData, "PHHoldings", "asset_cache")
	}
	return filepath.Join(os.TempDir(), "ph_holdings_assets")
}

// Helper functions

func getMimeType(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

func getExtensionFromMime(mimeType string) string {
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "application/pdf":
		return ".pdf"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".bin"
	}
}
