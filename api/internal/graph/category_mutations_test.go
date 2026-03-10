package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/commerce-projects/gitstore/api/internal/models"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCategory(t *testing.T) {
	t.Run("should create category with required fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCategoryInput{
			Name: "Electronics",
			Slug: "electronics",
		}

		payload, err := service.CreateCategory(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, payload)
		require.NotNil(t, payload.Category)

		category := payload.Category
		assert.Equal(t, "Electronics", category.Name)
		assert.Equal(t, "electronics", category.Slug)
		assert.Nil(t, category.ParentID)
		assert.Equal(t, 0, category.DisplayOrder)
		assert.NotEmpty(t, category.ID)
		assert.True(t, len(category.ID) > 4) // Should have prefix + base62
		assert.NotZero(t, category.CreatedAt)
		assert.NotZero(t, category.UpdatedAt)
	})

	t.Run("should create category with all fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create parent first
		parentInput := CreateCategoryInput{
			Name: "Electronics",
			Slug: "electronics",
		}
		parentPayload, err := service.CreateCategory(ctx, parentInput)
		require.NoError(t, err)
		parentID := parentPayload.Category.ID

		// Create child category
		displayOrder := 5
		body := "Laptops and notebooks for personal and business use."
		clientMutationID := "test-cat-123"

		input := CreateCategoryInput{
			ClientMutationID: &clientMutationID,
			Name:             "Laptops",
			Slug:             "laptops",
			ParentID:         &parentID,
			DisplayOrder:     &displayOrder,
			Body:             &body,
		}

		payload, err := service.CreateCategory(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, payload.ClientMutationID)
		assert.Equal(t, "test-cat-123", *payload.ClientMutationID)

		category := payload.Category
		assert.Equal(t, "Laptops", category.Name)
		assert.Equal(t, "laptops", category.Slug)
		assert.NotNil(t, category.ParentID)
		assert.Equal(t, parentID, *category.ParentID)
		assert.Equal(t, 5, category.DisplayOrder)
		assert.Contains(t, category.Body, "Laptops and notebooks")
	})

	t.Run("should create markdown file in repository", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCategoryInput{
			Name: "Books",
			Slug: "books",
		}

		_, err := service.CreateCategory(ctx, input)
		require.NoError(t, err)

		// Verify file was created
		filePath := filepath.Join(repoPath, "categories/books.md")
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)

		// Check file contains expected content
		contentStr := string(content)
		assert.Contains(t, contentStr, "---")
		assert.Contains(t, contentStr, "slug: books")
		assert.Contains(t, contentStr, "name: Books")
		assert.Contains(t, contentStr, "display_order: 0")
		assert.Contains(t, contentStr, "# Books")
	})

	t.Run("should commit file to git", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCategoryInput{
			Name: "Clothing",
			Slug: "clothing",
		}

		_, err := service.CreateCategory(ctx, input)
		require.NoError(t, err)

		// Verify git commit was created
		repo, err := git.PlainOpen(repoPath)
		require.NoError(t, err)

		ref, err := repo.Head()
		require.NoError(t, err)

		commit, err := repo.CommitObject(ref.Hash())
		require.NoError(t, err)

		assert.Contains(t, commit.Message, "create")
		assert.Contains(t, commit.Message, "category")
		assert.Contains(t, commit.Message, "clothing")
	})

	t.Run("should validate required fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		tests := []struct {
			name        string
			input       CreateCategoryInput
			expectedErr string
		}{
			{
				name: "missing name",
				input: CreateCategoryInput{
					Name: "",
					Slug: "test",
				},
				expectedErr: "name is required",
			},
			{
				name: "missing slug",
				input: CreateCategoryInput{
					Name: "Test",
					Slug: "",
				},
				expectedErr: "slug is required",
			},
			{
				name: "invalid slug with spaces",
				input: CreateCategoryInput{
					Name: "Test Category",
					Slug: "test category",
				},
				expectedErr: "lowercase alphanumeric",
			},
			{
				name: "invalid slug with uppercase",
				input: CreateCategoryInput{
					Name: "Test",
					Slug: "TestCategory",
				},
				expectedErr: "lowercase alphanumeric",
			},
			{
				name: "invalid slug with special chars",
				input: CreateCategoryInput{
					Name: "Test",
					Slug: "test_category",
				},
				expectedErr: "lowercase alphanumeric",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := service.CreateCategory(ctx, tt.input)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			})
		}
	})

	t.Run("should validate display order", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		negativeOrder := -1
		input := CreateCategoryInput{
			Name:         "Test",
			Slug:         "test",
			DisplayOrder: &negativeOrder,
		}

		_, err := service.CreateCategory(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "display order cannot be negative")
	})

	t.Run("should accept valid slug formats", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		validSlugs := []string{
			"electronics",
			"home-garden",
			"sports-outdoors",
			"electronics123",
			"cat123",
		}

		for i, slug := range validSlugs {
			input := CreateCategoryInput{
				Name: "Category " + slug,
				Slug: slug,
			}

			payload, err := service.CreateCategory(ctx, input)
			require.NoError(t, err, "Failed for slug: %s", slug)
			assert.Equal(t, slug, payload.Category.Slug)

			// Verify file exists
			filePath := filepath.Join(repoPath, "categories", slug+".md")
			_, err = os.Stat(filePath)
			require.NoError(t, err, "File not created for slug %d: %s", i, slug)
		}
	})

	t.Run("should use default values", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCategoryInput{
			Name: "Default Category",
			Slug: "default-category",
			// DisplayOrder, Body not provided
		}

		payload, err := service.CreateCategory(ctx, input)
		require.NoError(t, err)

		category := payload.Category
		assert.Equal(t, 0, category.DisplayOrder)
		assert.Empty(t, category.Body)
		assert.Nil(t, category.ParentID)
	})

	t.Run("should create root and child categories", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create root category
		rootInput := CreateCategoryInput{
			Name: "Electronics",
			Slug: "electronics",
		}

		rootPayload, err := service.CreateCategory(ctx, rootInput)
		require.NoError(t, err)
		rootID := rootPayload.Category.ID

		// Create child category
		childInput := CreateCategoryInput{
			Name:     "Computers",
			Slug:     "computers",
			ParentID: &rootID,
		}

		childPayload, err := service.CreateCategory(ctx, childInput)
		require.NoError(t, err)

		// Verify parent-child relationship
		assert.Nil(t, rootPayload.Category.ParentID)
		assert.NotNil(t, childPayload.Category.ParentID)
		assert.Equal(t, rootID, *childPayload.Category.ParentID)
	})
}

func TestUpdateCategory(t *testing.T) {
	t.Run("should update category fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create initial category
		createInput := CreateCategoryInput{
			Name: "Electronics",
			Slug: "electronics",
		}
		createPayload, err := service.CreateCategory(ctx, createInput)
		require.NoError(t, err)
		categoryID := createPayload.Category.ID

		// Mock readCategoryFromGit to return the created category
		service.readCategoryFromGit = func(id string) (*models.CategoryMutation, string, error) {
			if id == categoryID {
				content := service.generateCategoryContent(createPayload.Category)
				return createPayload.Category, content, nil
			}
			return nil, "", fmt.Errorf("category not found")
		}

		// Update the category
		newName := "Consumer Electronics"
		newSlug := "consumer-electronics"
		newDisplayOrder := 10
		newBody := "Latest consumer electronics and gadgets."

		versionChecker := NewVersionChecker()
		originalContent := service.generateCategoryContent(createPayload.Category)
		version := versionChecker.CalculateVersion(originalContent)

		updateInput := UpdateCategoryInput{
			ID:           categoryID,
			Name:         &newName,
			Slug:         &newSlug,
			DisplayOrder: &newDisplayOrder,
			Body:         &newBody,
			Version:      version,
		}

		payload, err := service.UpdateCategory(ctx, updateInput)
		require.NoError(t, err)
		require.NotNil(t, payload)
		require.Nil(t, payload.Conflict)

		category := payload.Category
		assert.Equal(t, "Consumer Electronics", category.Name)
		assert.Equal(t, "consumer-electronics", category.Slug)
		assert.Equal(t, 10, category.DisplayOrder)
		assert.Contains(t, category.Body, "Latest consumer electronics")
	})

	t.Run("should detect optimistic lock conflict", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create initial category
		createInput := CreateCategoryInput{
			Name: "Books",
			Slug: "books",
		}
		createPayload, err := service.CreateCategory(ctx, createInput)
		require.NoError(t, err)
		categoryID := createPayload.Category.ID

		// Simulate concurrent modification
		modifiedCategory := *createPayload.Category
		modifiedCategory.Name = "Literature"
		modifiedCategory.UpdatedAt = time.Now().UTC()

		service.readCategoryFromGit = func(id string) (*models.CategoryMutation, string, error) {
			if id == categoryID {
				content := service.generateCategoryContent(&modifiedCategory)
				return &modifiedCategory, content, nil
			}
			return nil, "", fmt.Errorf("category not found")
		}

		// Try to update with old version
		versionChecker := NewVersionChecker()
		oldContent := service.generateCategoryContent(createPayload.Category)
		oldVersion := versionChecker.CalculateVersion(oldContent)

		newName := "Reading Materials"
		updateInput := UpdateCategoryInput{
			ID:      categoryID,
			Name:    &newName,
			Version: oldVersion,
		}

		payload, err := service.UpdateCategory(ctx, updateInput)
		require.NoError(t, err)
		require.NotNil(t, payload)
		require.NotNil(t, payload.Conflict)
		require.Nil(t, payload.Category)

		assert.True(t, payload.Conflict.Detected)
		assert.NotEmpty(t, payload.Conflict.Diff)
	})

	t.Run("should handle slug change with file move", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create initial category
		createInput := CreateCategoryInput{
			Name: "Home & Garden",
			Slug: "home-garden",
		}
		createPayload, err := service.CreateCategory(ctx, createInput)
		require.NoError(t, err)
		categoryID := createPayload.Category.ID

		service.readCategoryFromGit = func(id string) (*models.CategoryMutation, string, error) {
			if id == categoryID {
				content := service.generateCategoryContent(createPayload.Category)
				return createPayload.Category, content, nil
			}
			return nil, "", fmt.Errorf("category not found")
		}

		// Change the slug
		newSlug := "home-and-garden"
		versionChecker := NewVersionChecker()
		originalContent := service.generateCategoryContent(createPayload.Category)
		version := versionChecker.CalculateVersion(originalContent)

		updateInput := UpdateCategoryInput{
			ID:      categoryID,
			Slug:    &newSlug,
			Version: version,
		}

		payload, err := service.UpdateCategory(ctx, updateInput)
		require.NoError(t, err)
		require.NotNil(t, payload)

		// Verify old file doesn't exist
		oldFilePath := filepath.Join(repoPath, "categories/home-garden.md")
		_, err = os.Stat(oldFilePath)
		assert.True(t, os.IsNotExist(err))

		// Verify new file exists
		newFilePath := filepath.Join(repoPath, "categories/home-and-garden.md")
		_, err = os.Stat(newFilePath)
		require.NoError(t, err)
	})

	t.Run("should validate updated fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create initial category
		createInput := CreateCategoryInput{
			Name: "Sports",
			Slug: "sports",
		}
		createPayload, err := service.CreateCategory(ctx, createInput)
		require.NoError(t, err)
		categoryID := createPayload.Category.ID

		service.readCategoryFromGit = func(id string) (*models.CategoryMutation, string, error) {
			if id == categoryID {
				content := service.generateCategoryContent(createPayload.Category)
				return createPayload.Category, content, nil
			}
			return nil, "", fmt.Errorf("category not found")
		}

		versionChecker := NewVersionChecker()
		originalContent := service.generateCategoryContent(createPayload.Category)
		version := versionChecker.CalculateVersion(originalContent)

		// Try invalid slug
		invalidSlug := "Sports & Outdoors"
		updateInput := UpdateCategoryInput{
			ID:      categoryID,
			Slug:    &invalidSlug,
			Version: version,
		}

		_, err = service.UpdateCategory(ctx, updateInput)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "lowercase alphanumeric")
	})
}

func TestDeleteCategory(t *testing.T) {
	t.Run("should delete category", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create category
		createInput := CreateCategoryInput{
			Name: "Toys",
			Slug: "toys",
		}
		createPayload, err := service.CreateCategory(ctx, createInput)
		require.NoError(t, err)
		categoryID := createPayload.Category.ID

		service.readCategoryFromGit = func(id string) (*models.CategoryMutation, string, error) {
			if id == categoryID {
				content := service.generateCategoryContent(createPayload.Category)
				return createPayload.Category, content, nil
			}
			return nil, "", fmt.Errorf("category not found")
		}

		// Delete the category
		deleteInput := DeleteCategoryInput{
			ID: categoryID,
		}

		payload, err := service.DeleteCategory(ctx, deleteInput)
		require.NoError(t, err)
		require.NotNil(t, payload)
		require.NotNil(t, payload.DeletedCategoryID)
		assert.Equal(t, categoryID, *payload.DeletedCategoryID)

		// Verify file was deleted
		filePath := filepath.Join(repoPath, "categories/toys.md")
		_, err = os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("should commit deletion to git", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create category
		createInput := CreateCategoryInput{
			Name: "Furniture",
			Slug: "furniture",
		}
		createPayload, err := service.CreateCategory(ctx, createInput)
		require.NoError(t, err)
		categoryID := createPayload.Category.ID

		service.readCategoryFromGit = func(id string) (*models.CategoryMutation, string, error) {
			if id == categoryID {
				content := service.generateCategoryContent(createPayload.Category)
				return createPayload.Category, content, nil
			}
			return nil, "", fmt.Errorf("category not found")
		}

		// Delete
		deleteInput := DeleteCategoryInput{
			ID: categoryID,
		}

		_, err = service.DeleteCategory(ctx, deleteInput)
		require.NoError(t, err)

		// Verify git commit
		repo, err := git.PlainOpen(repoPath)
		require.NoError(t, err)

		ref, err := repo.Head()
		require.NoError(t, err)

		commit, err := repo.CommitObject(ref.Hash())
		require.NoError(t, err)

		assert.Contains(t, commit.Message, "delete")
		assert.Contains(t, commit.Message, "category")
		assert.Contains(t, commit.Message, "furniture")
	})

	t.Run("should return clientMutationId", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		// Create category
		createInput := CreateCategoryInput{
			Name: "Automotive",
			Slug: "automotive",
		}
		createPayload, err := service.CreateCategory(ctx, createInput)
		require.NoError(t, err)
		categoryID := createPayload.Category.ID

		service.readCategoryFromGit = func(id string) (*models.CategoryMutation, string, error) {
			if id == categoryID {
				content := service.generateCategoryContent(createPayload.Category)
				return createPayload.Category, content, nil
			}
			return nil, "", fmt.Errorf("category not found")
		}

		clientID := "delete-category-123"
		deleteInput := DeleteCategoryInput{
			ClientMutationID: &clientID,
			ID:               categoryID,
		}

		payload, err := service.DeleteCategory(ctx, deleteInput)
		require.NoError(t, err)
		require.NotNil(t, payload.ClientMutationID)
		assert.Equal(t, "delete-category-123", *payload.ClientMutationID)
	})
}
