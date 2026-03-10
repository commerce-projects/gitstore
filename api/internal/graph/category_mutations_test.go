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
