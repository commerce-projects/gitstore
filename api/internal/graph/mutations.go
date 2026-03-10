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
	repoPath          string
	remoteURL         string
	readProductFromGit func(productID string) (*models.Product, string, error)
}

// NewProductMutationService creates a new product mutation service
func NewProductMutationService(repoPath, remoteURL string) *ProductMutationService {
	s := &ProductMutationService{
		repoPath:  repoPath,
		remoteURL: remoteURL,
	}
	// Set default implementation
	s.readProductFromGit = s.defaultReadProductFromGit
	return s
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

// UpdateProductInput represents the input for updating a product
type UpdateProductInput struct {
	ClientMutationID  *string
	ID                string
	SKU               *string
	Title             *string
	Body              *string
	Price             *float64
	Currency          *string
	InventoryStatus   *string
	InventoryQuantity *int
	CategoryID        *string
	CollectionIDs     []string
	Images            []string
	Metadata          map[string]interface{}
	Version           string // For optimistic locking
}

// UpdateProductPayload represents the payload returned from updateProduct
type UpdateProductPayload struct {
	ClientMutationID *string
	Product          *models.Product
	Conflict         *OptimisticLockConflict
}

// OptimisticLockConflict contains information about a version conflict
type OptimisticLockConflict struct {
	Detected        bool
	CurrentVersion  string
	AttemptedVersion string
	CurrentProduct  *models.Product
	Diff            string
}

// DeleteProductInput represents the input for deleting a product
type DeleteProductInput struct {
	ClientMutationID *string
	ID               string
}

// DeleteProductPayload represents the payload returned from deleteProduct
type DeleteProductPayload struct {
	ClientMutationID *string
	DeletedProductID *string
}

// CreateCategoryInput represents the input for creating a category
type CreateCategoryInput struct {
	ClientMutationID *string
	Name             string
	Slug             string
	ParentID         *string
	DisplayOrder     *int
	Body             *string
}

// CreateCategoryPayload represents the payload returned from createCategory
type CreateCategoryPayload struct {
	ClientMutationID *string
	Category         *models.CategoryMutation
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

// UpdateProduct updates an existing product with optimistic locking
func (s *ProductMutationService) UpdateProduct(ctx context.Context, input UpdateProductInput) (*UpdateProductPayload, error) {
	// Ensure repository exists
	if err := ensureRepoExists(s.repoPath); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Read existing product from git
	existingProduct, existingContent, err := s.readProductFromGit(input.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to read product: %w", err)
	}

	// Check optimistic lock version
	versionChecker := NewVersionChecker()
	if err := versionChecker.CheckVersion(input.Version, existingContent, "product", input.ID); err != nil {
		// Version mismatch - return conflict information
		if vme, ok := err.(*VersionMismatchError); ok {
			// Generate diff
			diffGen := NewDiffGenerator()

			// Create updated product to show what user wanted
			updatedProduct := s.applyUpdates(existingProduct, input)
			updatedContent := s.generateProductContent(updatedProduct)

			diffResult := diffGen.GenerateDiff(existingContent, updatedContent)

			return &UpdateProductPayload{
				ClientMutationID: input.ClientMutationID,
				Product:          nil, // Not updated due to conflict
				Conflict: &OptimisticLockConflict{
					Detected:         true,
					CurrentVersion:   vme.ActualVersion,
					AttemptedVersion: vme.ExpectedVersion,
					CurrentProduct:   existingProduct,
					Diff:             diffResult.FormatDiffForDisplay(),
				},
			}, nil
		}
		return nil, err
	}

	// Apply updates to product
	updatedProduct := s.applyUpdates(existingProduct, input)

	// Update timestamp
	updatedProduct.UpdatedAt = time.Now().UTC()

	// Validate updated product
	if err := updatedProduct.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate markdown content
	frontMatter := gitclient.ProductFrontMatter{
		ID:                updatedProduct.ID,
		SKU:               updatedProduct.SKU,
		Title:             updatedProduct.Title,
		Price:             updatedProduct.Price,
		Currency:          updatedProduct.Currency,
		InventoryStatus:   updatedProduct.InventoryStatus,
		InventoryQuantity: updatedProduct.InventoryQuantity,
		CategoryID:        updatedProduct.CategoryID,
		CollectionIDs:     updatedProduct.CollectionIDs,
		Images:            updatedProduct.Images,
		Metadata:          updatedProduct.Metadata,
		CreatedAt:         updatedProduct.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         updatedProduct.UpdatedAt.Format(time.RFC3339),
	}

	markdown, err := gitclient.GenerateProductMarkdown(frontMatter, updatedProduct.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to generate markdown: %w", err)
	}

	// Determine file path (may have changed if category changed)
	categorySlug := models.GetCategorySlug(updatedProduct.CategoryID)
	filePath := gitclient.GetProductFilePath(updatedProduct.SKU, categorySlug)

	// Check if file path changed (category or SKU changed)
	oldCategorySlug := models.GetCategorySlug(existingProduct.CategoryID)
	oldFilePath := gitclient.GetProductFilePath(existingProduct.SKU, oldCategorySlug)

	// Commit the changes
	commitBuilder, err := gitclient.NewCommitBuilder(s.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git: %w", err)
	}

	var commitHash string
	if oldFilePath != filePath {
		// File moved - delete old and write new
		changes := map[string]string{
			filePath: markdown,
		}

		// Delete old file
		if err := commitBuilder.DeleteFile(oldFilePath); err != nil {
			return nil, fmt.Errorf("failed to delete old file: %w", err)
		}

		commitMsg := gitclient.GenerateCommitMessage("update", "product", updatedProduct.SKU,
			fmt.Sprintf("%s (moved from %s)", updatedProduct.Title, existingProduct.SKU))
		commitHash, err = commitBuilder.CommitMultiple(changes, commitMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
	} else {
		// Simple update
		commitMsg := gitclient.GenerateCommitMessage("update", "product", updatedProduct.SKU, updatedProduct.Title)
		commitHash, err = commitBuilder.CommitChange(filePath, markdown, commitMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
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
	fmt.Printf("Updated product %s (commit: %s)\n", updatedProduct.SKU, commitHash[:8])

	return &UpdateProductPayload{
		ClientMutationID: input.ClientMutationID,
		Product:          updatedProduct,
		Conflict:         nil,
	}, nil
}

// defaultReadProductFromGit is the default implementation for reading products
func (s *ProductMutationService) defaultReadProductFromGit(productID string) (*models.Product, string, error) {
	// For now, we need to search for the product file
	// In a real implementation, we'd have an index or cache
	// For testing, we'll use a simplified approach

	// This is a placeholder - in reality, you'd need to:
	// 1. Have a cache/index of products
	// 2. Know the file path from the index
	// 3. Read the file and parse it

	return nil, "", fmt.Errorf("product not found: %s (cache/index not yet implemented)", productID)
}

// applyUpdates applies the update input to an existing product
func (s *ProductMutationService) applyUpdates(existing *models.Product, input UpdateProductInput) *models.Product {
	updated := &models.Product{
		ID:                existing.ID,
		SKU:               existing.SKU,
		Title:             existing.Title,
		Body:              existing.Body,
		Price:             existing.Price,
		Currency:          existing.Currency,
		InventoryStatus:   existing.InventoryStatus,
		InventoryQuantity: existing.InventoryQuantity,
		CategoryID:        existing.CategoryID,
		CollectionIDs:     existing.CollectionIDs,
		Images:            existing.Images,
		Metadata:          existing.Metadata,
		CreatedAt:         existing.CreatedAt,
		UpdatedAt:         existing.UpdatedAt,
	}

	// Apply updates only for provided fields
	if input.SKU != nil {
		updated.SKU = *input.SKU
	}
	if input.Title != nil {
		updated.Title = *input.Title
	}
	if input.Body != nil {
		updated.Body = *input.Body
	}
	if input.Price != nil {
		updated.Price = *input.Price
	}
	if input.Currency != nil {
		updated.Currency = *input.Currency
	}
	if input.InventoryStatus != nil {
		updated.InventoryStatus = *input.InventoryStatus
	}
	if input.InventoryQuantity != nil {
		updated.InventoryQuantity = input.InventoryQuantity
	}
	if input.CategoryID != nil {
		updated.CategoryID = *input.CategoryID
	}
	if input.CollectionIDs != nil {
		updated.CollectionIDs = input.CollectionIDs
	}
	if input.Images != nil {
		updated.Images = input.Images
	}
	if input.Metadata != nil {
		updated.Metadata = input.Metadata
	}

	return updated
}

// generateProductContent generates the full markdown content for a product
func (s *ProductMutationService) generateProductContent(product *models.Product) string {
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

	markdown, _ := gitclient.GenerateProductMarkdown(frontMatter, product.Body)
	return markdown
}

// DeleteProduct deletes an existing product
func (s *ProductMutationService) DeleteProduct(ctx context.Context, input DeleteProductInput) (*DeleteProductPayload, error) {
	// Ensure repository exists
	if err := ensureRepoExists(s.repoPath); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Read existing product from git
	existingProduct, _, err := s.readProductFromGit(input.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to read product: %w", err)
	}

	// Determine file path
	categorySlug := models.GetCategorySlug(existingProduct.CategoryID)
	filePath := gitclient.GetProductFilePath(existingProduct.SKU, categorySlug)

	// Commit the deletion
	commitBuilder, err := gitclient.NewCommitBuilder(s.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git: %w", err)
	}

	commitMsg := gitclient.GenerateCommitMessage("delete", "product", existingProduct.SKU, existingProduct.Title)
	commitHash, err := commitBuilder.CommitDelete(filePath, commitMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to commit deletion: %w", err)
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
	fmt.Printf("Deleted product %s (commit: %s)\n", existingProduct.SKU, commitHash[:8])

	return &DeleteProductPayload{
		ClientMutationID: input.ClientMutationID,
		DeletedProductID: &input.ID,
	}, nil
}

// CreateCategory creates a new category and commits it to git
func (s *ProductMutationService) CreateCategory(ctx context.Context, input CreateCategoryInput) (*CreateCategoryPayload, error) {
	// Set defaults
	displayOrder := 0
	if input.DisplayOrder != nil {
		displayOrder = *input.DisplayOrder
	}

	body := ""
	if input.Body != nil {
		body = *input.Body
	}

	// Create category model
	category, err := models.NewCategory(input.Name, input.Slug, input.ParentID, displayOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	category.Body = body

	// Validate category
	if err := category.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Ensure repository exists
	if err := ensureRepoExists(s.repoPath); err != nil {
		return nil, fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Generate markdown content
	frontMatter := gitclient.CategoryFrontMatter{
		ID:           category.ID,
		Name:         category.Name,
		Slug:         category.Slug,
		ParentID:     category.ParentID,
		DisplayOrder: category.DisplayOrder,
		CreatedAt:    category.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    category.UpdatedAt.Format(time.RFC3339),
	}

	// Add description if body is provided
	var description *string
	if body != "" {
		description = &body
		frontMatter.Description = description
	}

	markdown, err := gitclient.GenerateCategoryMarkdown(frontMatter, body)
	if err != nil {
		return nil, fmt.Errorf("failed to generate markdown: %w", err)
	}

	// Determine file path
	filePath := gitclient.GetCategoryFilePath(category.Slug)

	// Commit the file
	commitBuilder, err := gitclient.NewCommitBuilder(s.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git: %w", err)
	}

	commitMsg := gitclient.GenerateCommitMessage("create", "category", category.Slug, category.Name)
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
	fmt.Printf("Created category %s (commit: %s)\n", category.Slug, commitHash[:8])

	return &CreateCategoryPayload{
		ClientMutationID: input.ClientMutationID,
		Category:         category,
	}, nil
}
