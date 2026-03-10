package graph

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestMutationRepo(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "gitstore-mutations-test-*")
	require.NoError(t, err)

	// Initialize git repository
	_, err = git.PlainInit(tmpDir, false)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestCreateProduct(t *testing.T) {
	t.Run("should create product with all required fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateProductInput{
			SKU:        "TEST-PRODUCT-001",
			Title:      "Test Product",
			Price:      29.99,
			CategoryID: "cat_electronics",
		}

		payload, err := service.CreateProduct(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, payload)
		require.NotNil(t, payload.Product)

		product := payload.Product
		assert.Equal(t, "TEST-PRODUCT-001", product.SKU)
		assert.Equal(t, "Test Product", product.Title)
		assert.Equal(t, 29.99, product.Price)
		assert.Equal(t, "USD", product.Currency)
		assert.Equal(t, "IN_STOCK", product.InventoryStatus)
		assert.Equal(t, "cat_electronics", product.CategoryID)
		assert.NotEmpty(t, product.ID)
		assert.True(t, len(product.ID) > 5) // Should have prefix + base62
		assert.NotZero(t, product.CreatedAt)
		assert.NotZero(t, product.UpdatedAt)
	})

	t.Run("should create product with optional fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		currency := "EUR"
		inventoryStatus := "OUT_OF_STOCK"
		inventoryQuantity := 50
		body := "# Product Description\n\nThis is the product body."
		clientMutationID := "test-mutation-123"

		input := CreateProductInput{
			ClientMutationID:  &clientMutationID,
			SKU:               "TEST-002",
			Title:             "Product with Options",
			Body:              &body,
			Price:             99.99,
			Currency:          &currency,
			InventoryStatus:   &inventoryStatus,
			InventoryQuantity: &inventoryQuantity,
			CategoryID:        "cat_accessories",
			CollectionIDs:     []string{"coll_featured", "coll_bestsellers"},
			Images:            []string{"https://cdn.example.com/image.jpg"},
			Metadata: map[string]interface{}{
				"brand":  "TestBrand",
				"weight": 1.5,
			},
		}

		payload, err := service.CreateProduct(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, payload)
		assert.Equal(t, "test-mutation-123", *payload.ClientMutationID)

		product := payload.Product
		assert.Equal(t, "EUR", product.Currency)
		assert.Equal(t, "OUT_OF_STOCK", product.InventoryStatus)
		assert.Equal(t, 50, *product.InventoryQuantity)
		assert.Contains(t, product.Body, "Product Description")
		assert.Len(t, product.CollectionIDs, 2)
		assert.Contains(t, product.CollectionIDs, "coll_featured")
		assert.Len(t, product.Images, 1)
		assert.Equal(t, "TestBrand", product.Metadata["brand"])
	})

	t.Run("should create markdown file in repository", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateProductInput{
			SKU:        "LAPTOP-001",
			Title:      "Premium Laptop",
			Price:      1299.99,
			CategoryID: "cat_electronics",
		}

		_, err := service.CreateProduct(ctx, input)
		require.NoError(t, err)

		// Verify file was created
		filePath := filepath.Join(repoPath, "products/electronics/LAPTOP-001.md")
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)

		// Check file contains expected content
		contentStr := string(content)
		assert.Contains(t, contentStr, "---")
		assert.Contains(t, contentStr, "sku: LAPTOP-001")
		assert.Contains(t, contentStr, "title: Premium Laptop")
		assert.Contains(t, contentStr, "price: 1299.99")
		assert.Contains(t, contentStr, "# Premium Laptop")
	})

	t.Run("should commit file to git", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateProductInput{
			SKU:        "WIDGET-001",
			Title:      "Test Widget",
			Price:      19.99,
			CategoryID: "cat_widgets",
		}

		_, err := service.CreateProduct(ctx, input)
		require.NoError(t, err)

		// Verify git commit was created
		repo, err := git.PlainOpen(repoPath)
		require.NoError(t, err)

		ref, err := repo.Head()
		require.NoError(t, err)

		commit, err := repo.CommitObject(ref.Hash())
		require.NoError(t, err)

		assert.Contains(t, commit.Message, "create")
		assert.Contains(t, commit.Message, "product")
		assert.Contains(t, commit.Message, "WIDGET-001")
	})

	t.Run("should validate required fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		tests := []struct {
			name        string
			input       CreateProductInput
			expectedErr string
		}{
			{
				name: "missing SKU",
				input: CreateProductInput{
					SKU:        "",
					Title:      "Test",
					Price:      10.00,
					CategoryID: "cat_test",
				},
				expectedErr: "SKU is required",
			},
			{
				name: "missing title",
				input: CreateProductInput{
					SKU:        "TEST-001",
					Title:      "",
					Price:      10.00,
					CategoryID: "cat_test",
				},
				expectedErr: "title is required",
			},
			{
				name: "negative price",
				input: CreateProductInput{
					SKU:        "TEST-001",
					Title:      "Test",
					Price:      -10.00,
					CategoryID: "cat_test",
				},
				expectedErr: "price cannot be negative",
			},
			{
				name: "missing category",
				input: CreateProductInput{
					SKU:        "TEST-001",
					Title:      "Test",
					Price:      10.00,
					CategoryID: "",
				},
				expectedErr: "categoryID is required",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := service.CreateProduct(ctx, tt.input)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			})
		}
	})

	t.Run("should validate inventory status", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		invalidStatus := "INVALID_STATUS"
		input := CreateProductInput{
			SKU:             "TEST-001",
			Title:           "Test",
			Price:           10.00,
			CategoryID:      "cat_test",
			InventoryStatus: &invalidStatus,
		}

		_, err := service.CreateProduct(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid inventory status")
	})

	t.Run("should validate SKU format", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		tests := []struct {
			sku         string
			shouldError bool
			errorMsg    string
		}{
			{"AB", true, "at least 3 characters"},
			{"VALID-SKU-123", false, ""},
			{"SKU_WITH_UNDERSCORE", false, ""},
			{"SKU@WITH@SPECIAL", true, "alphanumeric"},
			{"SKU WITH SPACES", true, "alphanumeric"},
		}

		for _, tt := range tests {
			input := CreateProductInput{
				SKU:        tt.sku,
				Title:      "Test Product",
				Price:      10.00,
				CategoryID: "cat_test",
			}

			_, err := service.CreateProduct(ctx, input)
			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		}
	})

	t.Run("should use default values", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateProductInput{
			SKU:        "TEST-DEFAULTS",
			Title:      "Test Defaults",
			Price:      10.00,
			CategoryID: "cat_test",
			// Currency, InventoryStatus not provided
		}

		payload, err := service.CreateProduct(ctx, input)
		require.NoError(t, err)

		product := payload.Product
		assert.Equal(t, "USD", product.Currency)
		assert.Equal(t, "IN_STOCK", product.InventoryStatus)
		assert.Empty(t, product.CollectionIDs)
		assert.Empty(t, product.Images)
		assert.Empty(t, product.Metadata)
	})

	t.Run("should handle uncategorized products", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateProductInput{
			SKU:        "UNCAT-001",
			Title:      "Uncategorized Product",
			Price:      5.00,
			CategoryID: "cat_",
		}

		_, err := service.CreateProduct(ctx, input)
		require.NoError(t, err)

		// Should create file in uncategorized folder
		filePath := filepath.Join(repoPath, "products/uncategorized/UNCAT-001.md")
		_, err = os.Stat(filePath)
		require.NoError(t, err)
	})
}

func TestEnsureRepoExists(t *testing.T) {
	t.Run("should succeed for existing git repo", func(t *testing.T) {
		tmpDir, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		err := ensureRepoExists(tmpDir)
		assert.NoError(t, err)
	})

	t.Run("should fail for non-git directory", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gitstore-not-git-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		err = ensureRepoExists(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git repository not initialized")
	})

	t.Run("should create directory if it doesn't exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "gitstore-parent-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		newRepoPath := filepath.Join(tmpDir, "new-repo")

		// Should create the directory
		err = ensureRepoExists(newRepoPath)
		// Will fail because .git doesn't exist, but directory should be created
		assert.Error(t, err) // Expected - no git init yet

		// Check directory was created
		_, err = os.Stat(newRepoPath)
		assert.NoError(t, err)
	})
}
