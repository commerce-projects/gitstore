package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/commerce-projects/gitstore/api/internal/gitclient"
	"github.com/commerce-projects/gitstore/api/internal/models"
)

// ProductMutationService handles product mutation operations
type ProductMutationService struct {
	repoPath  string
	remoteURL string
}

// NewProductMutationService creates a new product mutation service
func NewProductMutationService(repoPath, remoteURL string) *ProductMutationService {
	return &ProductMutationService{
		repoPath:  repoPath,
		remoteURL: remoteURL,
	}
}

// CreateProductInput represents the input for creating a product
type CreateProductInput struct {
	ClientMutationID  *string
	SKU               string
	Title             string
	Body              *string
	Price             float64
	Currency          *string
	InventoryStatus   *string
	InventoryQuantity *int
	CategoryID        string
	CollectionIDs     []string
	Images            []string
	Metadata          map[string]interface{}
}

// CreateProductPayload represents the payload returned from createProduct
type CreateProductPayload struct {
	ClientMutationID *string
	Product          *models.Product
}

// CreateProduct creates a new product and commits it to git
func (s *ProductMutationService) CreateProduct(ctx context.Context, input CreateProductInput) (*CreateProductPayload, error) {
	// Set defaults
	currency := "USD"
	if input.Currency != nil {
		currency = *input.Currency
	}

	inventoryStatus := "IN_STOCK"
	if input.InventoryStatus != nil {
		inventoryStatus = *input.InventoryStatus
	}

	body := ""
	if input.Body != nil {
		body = *input.Body
	}

	// Create product model
	product, err := models.NewProduct(
		input.SKU,
		input.Title,
		body,
		input.Price,
		currency,
		input.CategoryID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	// Set optional fields
	product.InventoryStatus = inventoryStatus
	product.InventoryQuantity = input.InventoryQuantity

	if input.CollectionIDs != nil {
		product.CollectionIDs = input.CollectionIDs
	}

	if input.Images != nil {
		product.Images = input.Images
	}

	if input.Metadata != nil {
		product.Metadata = input.Metadata
	}

	// Validate product
	if err := product.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Ensure repository exists
	if err := ensureRepoExists(s.repoPath); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Generate markdown content
	frontMatter := gitclient.ProductFrontMatter{
		ID:                product.ID,
		SKU:               product.SKU,
		Title:             product.Title,
		Price:             product.Price,
		Currency:          product.Currency,
		InventoryStatus:   product.InventoryStatus,
		InventoryQuantity: product.InventoryQuantity,
		CategoryID:        product.CategoryID,
		CollectionIDs:     product.CollectionIDs,
		Images:            product.Images,
		Metadata:          product.Metadata,
		CreatedAt:         product.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         product.UpdatedAt.Format(time.RFC3339),
	}

	markdown, err := gitclient.GenerateProductMarkdown(frontMatter, product.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to generate markdown: %w", err)
	}

	// Determine file path
	categorySlug := models.GetCategorySlug(product.CategoryID)
	filePath := gitclient.GetProductFilePath(product.SKU, categorySlug)

	// Commit the file
	commitBuilder, err := gitclient.NewCommitBuilder(s.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git: %w", err)
	}

	commitMsg := gitclient.GenerateCommitMessage("create", "product", product.SKU, product.Title)
	commitHash, err := commitBuilder.CommitChange(filePath, markdown, commitMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	// Push to remote (if configured)
	if s.remoteURL != "" {
		pushClient, err := gitclient.NewPushClient(s.repoPath, "origin", s.remoteURL)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize push client: %w", err)
		}

		if err := pushClient.PushBranch(); err != nil {
			return nil, fmt.Errorf("failed to push to remote: %w", err)
		}
	}

	// Log success
	fmt.Printf("Created product %s (commit: %s)\n", product.SKU, commitHash[:8])

	return &CreateProductPayload{
		ClientMutationID: input.ClientMutationID,
		Product:          product,
	}, nil
}

// ensureRepoExists creates the repository directory if it doesn't exist
func ensureRepoExists(repoPath string) error {
	// Check if directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Create directory
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			return fmt.Errorf("failed to create repo directory: %w", err)
		}

		// Initialize git repository
		// For now, we'll just create the directory
		// The CommitBuilder will handle git init if needed
	} else if err != nil {
		return fmt.Errorf("failed to check repo directory: %w", err)
	}

	// Ensure .git directory exists (basic check)
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Need to initialize git repository
		// CommitBuilder expects an existing repo, so we should initialize it here
		return fmt.Errorf("git repository not initialized at %s (run 'git init' first)", repoPath)
	}

	return nil
}
