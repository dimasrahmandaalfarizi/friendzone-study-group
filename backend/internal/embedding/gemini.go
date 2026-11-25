package embedding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/genai"
)

const (
	defaultModel     = "gemini-embedding-001"
	outputDimensions = int32(768)
)

type GeminiEmbedder struct {
	client *genai.Client
}

func NewGeminiEmbedder(ctx context.Context, apiKey string) (*GeminiEmbedder, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create genai client: %w", err)
	}

	return &GeminiEmbedder{
		client: client,
	}, nil
}

func (e *GeminiEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dim := outputDimensions
	res, err := e.client.Models.EmbedContent(
		reqCtx,
		defaultModel,
		genai.Text(text),
		&genai.EmbedContentConfig{
			OutputDimensionality: &dim,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("call gemini embedding api: %w", err)
	}

	if len(res.Embeddings) == 0 || len(res.Embeddings[0].Values) == 0 {
		return nil, errors.New("empty embedding response from gemini api")
	}

	if len(res.Embeddings[0].Values) != int(outputDimensions) {
		return nil, fmt.Errorf("unexpected embedding dimension: got %d, want %d", len(res.Embeddings[0].Values), outputDimensions)
	}

	return res.Embeddings[0].Values, nil
}