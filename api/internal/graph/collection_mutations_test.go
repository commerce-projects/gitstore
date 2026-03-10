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

func TestCreateCollection(t *testing.T) {
	t.Run("should create collection with required fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCollectionInput{
			Name: "Summer Sale",
			Slug: "summer-sale",
		}

		payload, err := service.CreateCollection(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, payload)
		require.NotNil(t, payload.Collection)

		collection := payload.Collection
		assert.Equal(t, "Summer Sale", collection.Name)
		assert.Equal(t, "summer-sale", collection.Slug)
		assert.Equal(t, 0, collection.DisplayOrder)
		assert.NotEmpty(t, collection.ID)
		assert.True(t, len(collection.ID) > 4) // Should have prefix + base62
		assert.NotZero(t, collection.CreatedAt)
		assert.NotZero(t, collection.UpdatedAt)
	})

	t.Run("should create collection with all fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		displayOrder := 5
		body := "Best products from our summer collection. Limited time offer!"
		clientMutationID := "test-col-123"

		input := CreateCollectionInput{
			ClientMutationID: &clientMutationID,
			Name:             "Summer Sale 2026",
			Slug:             "summer-sale-2026",
			DisplayOrder:     &displayOrder,
			Body:             &body,
		}

		payload, err := service.CreateCollection(ctx, input)
		require.NoError(t, err)
		require.NotNil(t, payload.ClientMutationID)
		assert.Equal(t, "test-col-123", *payload.ClientMutationID)

		collection := payload.Collection
		assert.Equal(t, "Summer Sale 2026", collection.Name)
		assert.Equal(t, "summer-sale-2026", collection.Slug)
		assert.Equal(t, 5, collection.DisplayOrder)
		assert.Contains(t, collection.Body, "Best products")
	})

	t.Run("should create markdown file in repository", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCollectionInput{
			Name: "New Arrivals",
			Slug: "new-arrivals",
		}

		_, err := service.CreateCollection(ctx, input)
		require.NoError(t, err)

		// Verify file was created
		filePath := filepath.Join(repoPath, "collections/new-arrivals.md")
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)

		// Check file contains expected content
		contentStr := string(content)
		assert.Contains(t, contentStr, "---")
		assert.Contains(t, contentStr, "slug: new-arrivals")
		assert.Contains(t, contentStr, "name: New Arrivals")
		assert.Contains(t, contentStr, "display_order: 0")
		assert.Contains(t, contentStr, "# New Arrivals")
	})

	t.Run("should commit file to git", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCollectionInput{
			Name: "Best Sellers",
			Slug: "best-sellers",
		}

		_, err := service.CreateCollection(ctx, input)
		require.NoError(t, err)

		// Verify git commit was created
		repo, err := git.PlainOpen(repoPath)
		require.NoError(t, err)

		ref, err := repo.Head()
		require.NoError(t, err)

		commit, err := repo.CommitObject(ref.Hash())
		require.NoError(t, err)

		assert.Contains(t, commit.Message, "create")
		assert.Contains(t, commit.Message, "collection")
		assert.Contains(t, commit.Message, "best-sellers")
	})

	t.Run("should validate required fields", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		tests := []struct {
			name        string
			input       CreateCollectionInput
			expectedErr string
		}{
			{
				name: "missing name",
				input: CreateCollectionInput{
					Name: "",
					Slug: "test",
				},
				expectedErr: "name is required",
			},
			{
				name: "missing slug",
				input: CreateCollectionInput{
					Name: "Test",
					Slug: "",
				},
				expectedErr: "slug is required",
			},
			{
				name: "invalid slug with spaces",
				input: CreateCollectionInput{
					Name: "Test Collection",
					Slug: "test collection",
				},
				expectedErr: "lowercase alphanumeric",
			},
			{
				name: "invalid slug with uppercase",
				input: CreateCollectionInput{
					Name: "Test",
					Slug: "TestCollection",
				},
				expectedErr: "lowercase alphanumeric",
			},
			{
				name: "invalid slug with special chars",
				input: CreateCollectionInput{
					Name: "Test",
					Slug: "test_collection",
				},
				expectedErr: "lowercase alphanumeric",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := service.CreateCollection(ctx, tt.input)
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
		input := CreateCollectionInput{
			Name:         "Test",
			Slug:         "test",
			DisplayOrder: &negativeOrder,
		}

		_, err := service.CreateCollection(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "display order cannot be negative")
	})

	t.Run("should accept valid slug formats", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		validSlugs := []string{
			"sale",
			"summer-sale",
			"spring-2026",
			"collection123",
			"col123",
		}

		for i, slug := range validSlugs {
			input := CreateCollectionInput{
				Name: "Collection " + slug,
				Slug: slug,
			}

			payload, err := service.CreateCollection(ctx, input)
			require.NoError(t, err, "Failed for slug: %s", slug)
			assert.Equal(t, slug, payload.Collection.Slug)

			// Verify file exists
			filePath := filepath.Join(repoPath, "collections", slug+".md")
			_, err = os.Stat(filePath)
			require.NoError(t, err, "File not created for slug %d: %s", i, slug)
		}
	})

	t.Run("should use default values", func(t *testing.T) {
		repoPath, cleanup := setupTestMutationRepo(t)
		defer cleanup()

		service := NewProductMutationService(repoPath, "")
		ctx := context.Background()

		input := CreateCollectionInput{
			Name: "Default Collection",
			Slug: "default-collection",
			// DisplayOrder, Body not provided
		}

		payload, err := service.CreateCollection(ctx, input)
		require.NoError(t, err)

		collection := payload.Collection
		assert.Equal(t, 0, collection.DisplayOrder)
		assert.Empty(t, collection.Body)
	})
}
