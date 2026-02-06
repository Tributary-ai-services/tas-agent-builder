package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tas-agent-builder/config"
	"github.com/tas-agent-builder/models"
	"github.com/tas-agent-builder/services"
)

// NotebookServiceImpl implements the NotebookService interface by calling Aether-BE internal API
type NotebookServiceImpl struct {
	config     *config.AetherConfig
	httpClient *http.Client
}

// NewNotebookService creates a new NotebookService implementation
func NewNotebookService(cfg *config.AetherConfig) services.NotebookService {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &NotebookServiceImpl{
		config: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// AetherNotebookHierarchy represents the response from Aether-BE hierarchy endpoint
type AetherNotebookHierarchy struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	ParentID     string                    `json:"parent_id,omitempty"`
	Documents    []AetherNotebookDocument  `json:"documents,omitempty"`
	SubNotebooks []AetherNotebookHierarchy `json:"sub_notebooks,omitempty"`
}

// AetherNotebookDocument represents a document response from Aether-BE
type AetherNotebookDocument struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	NotebookID   string `json:"notebook_id"`
	NotebookName string `json:"notebook_name,omitempty"`
	FileID       string `json:"file_id,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	ChunkCount   int    `json:"chunk_count,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
}

// AetherDocumentsResponse represents the response from Aether-BE documents endpoint
type AetherDocumentsResponse struct {
	NotebookID string                   `json:"notebook_id"`
	Documents  []AetherNotebookDocument `json:"documents"`
	Total      int                      `json:"total"`
}

// AetherSubNotebooksResponse represents the response from Aether-BE sub-notebooks endpoint
type AetherSubNotebooksResponse struct {
	ParentNotebookID string   `json:"parent_notebook_id"`
	SubNotebookIDs   []string `json:"sub_notebook_ids"`
	Total            int      `json:"total"`
}

// GetNotebookHierarchy retrieves notebook hierarchy including sub-notebooks
func (s *NotebookServiceImpl) GetNotebookHierarchy(ctx context.Context, notebookID uuid.UUID, tenantID string) (*models.NotebookHierarchy, error) {
	url := fmt.Sprintf("%s/api/v1/internal/notebooks/%s/hierarchy?tenant_id=%s",
		s.config.BaseURL, notebookID.String(), tenantID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Aether-BE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Aether-BE returned status %d", resp.StatusCode)
	}

	var aetherHierarchy AetherNotebookHierarchy
	if err := json.NewDecoder(resp.Body).Decode(&aetherHierarchy); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertAetherHierarchy(&aetherHierarchy), nil
}

// GetDocumentsRecursive retrieves all documents from a notebook and its sub-notebooks
func (s *NotebookServiceImpl) GetDocumentsRecursive(ctx context.Context, notebookID uuid.UUID, tenantID string) ([]models.NotebookDocument, error) {
	url := fmt.Sprintf("%s/api/v1/internal/notebooks/%s/documents/recursive?tenant_id=%s",
		s.config.BaseURL, notebookID.String(), tenantID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Aether-BE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Aether-BE returned status %d", resp.StatusCode)
	}

	var aetherResp AetherDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&aetherResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertAetherDocuments(aetherResp.Documents), nil
}

// GetSubNotebookIDs retrieves IDs of all sub-notebooks for a parent notebook
func (s *NotebookServiceImpl) GetSubNotebookIDs(ctx context.Context, parentNotebookID uuid.UUID, tenantID string) ([]uuid.UUID, error) {
	url := fmt.Sprintf("%s/api/v1/internal/notebooks/%s/sub-notebooks?tenant_id=%s",
		s.config.BaseURL, parentNotebookID.String(), tenantID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if s.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Aether-BE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Aether-BE returned status %d", resp.StatusCode)
	}

	var aetherResp AetherSubNotebooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&aetherResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert string IDs to UUIDs
	var subNotebookIDs []uuid.UUID
	for _, idStr := range aetherResp.SubNotebookIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue // Skip invalid UUIDs
		}
		subNotebookIDs = append(subNotebookIDs, id)
	}

	return subNotebookIDs, nil
}

// Helper function to convert Aether hierarchy to models hierarchy
func convertAetherHierarchy(aether *AetherNotebookHierarchy) *models.NotebookHierarchy {
	if aether == nil {
		return nil
	}

	id, _ := uuid.Parse(aether.ID)
	var parentID *uuid.UUID
	if aether.ParentID != "" {
		pid, err := uuid.Parse(aether.ParentID)
		if err == nil {
			parentID = &pid
		}
	}

	hierarchy := &models.NotebookHierarchy{
		ID:        id,
		Name:      aether.Name,
		ParentID:  parentID,
		Documents: convertAetherDocuments(aether.Documents),
	}

	// Convert sub-notebooks recursively
	for _, sub := range aether.SubNotebooks {
		subHierarchy := convertAetherHierarchy(&sub)
		if subHierarchy != nil {
			hierarchy.SubNotebooks = append(hierarchy.SubNotebooks, *subHierarchy)
		}
	}

	return hierarchy
}

// Helper function to convert Aether documents to model documents
func convertAetherDocuments(aetherDocs []AetherNotebookDocument) []models.NotebookDocument {
	var documents []models.NotebookDocument
	for _, ad := range aetherDocs {
		id, _ := uuid.Parse(ad.ID)
		notebookID, _ := uuid.Parse(ad.NotebookID)
		fileID, _ := uuid.Parse(ad.FileID)

		doc := models.NotebookDocument{
			ID:           id,
			Name:         ad.Name,
			NotebookID:   notebookID,
			NotebookName: ad.NotebookName,
			FileID:       fileID,
			ContentType:  ad.ContentType,
			SizeBytes:    ad.SizeBytes,
			ChunkCount:   ad.ChunkCount,
		}
		documents = append(documents, doc)
	}
	return documents
}
