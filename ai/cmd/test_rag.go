package main

import (
	"context"
	compRag "dubbo-admin-ai/component/rag"
	appconfig "dubbo-admin-ai/config"
	"dubbo-admin-ai/runtime"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/openai/openai-go/option"
	"gopkg.in/yaml.v3"
)

type TestRAGCommand struct {
	Query      string
	ConfigPath string
}

func main() {
	query := "Dubbo的负载均衡策略有哪些"
	configPath := "component/rag/rag.yaml"

	if len(os.Args) > 1 {
		query = os.Args[1]
	}
	if len(os.Args) > 2 {
		configPath = os.Args[2]
	}

	cmd := &TestRAGCommand{
		Query:      query,
		ConfigPath: configPath,
	}

	if err := executeTest(cmd); err != nil {
		log.Fatalf("Test failed: %v", err)
	}
}

func executeTest(cmd *TestRAGCommand) error {
	ctx := context.Background()

	// Load RAG configuration
	cfg, err := loadRAGConfig(cmd.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid rag config: %w", err)
	}

	// Initialize plugins
	var dashscopePlugin *compat_oai.OpenAICompatible
	plugins := []api.Plugin{}

	if key := os.Getenv("DASHSCOPE_API_KEY"); key != "" {
		dashscopePlugin = &compat_oai.OpenAICompatible{
			Provider: "dashscope",
			Opts: []option.RequestOption{
				option.WithAPIKey(key),
				option.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1"),
			},
		}
		plugins = append(plugins, dashscopePlugin)
	}

	g := genkit.Init(ctx, genkit.WithPlugins(plugins...))

	// Define embedder
	if dashscopePlugin != nil {
		embedder := dashscopePlugin.DefineEmbedder("dashscope", "text-embedding-v4", &ai.EmbedderOptions{
			Label:      "qwen3-embedding",
			Supports:   &ai.EmbedderSupports{Input: []string{"text"}},
			Dimensions: 1024,
		})
		genkit.RegisterAction(g, embedder)
	}

	// Build RAG system
	sys, err := buildRAGSystem(g, cfg)
	if err != nil {
		return fmt.Errorf("failed to build RAG system: %w", err)
	}

	fmt.Printf("=== RAG Test ===\n")
	fmt.Printf("Query: %s\n\n", cmd.Query)

	// Test retrieval
	req := &compRag.RetrieveRequest{
		Query: cmd.Query,
		TopK:  5,
	}

	resp, err := sys.RetrieveV2(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to retrieve: %w", err)
	}

	fmt.Printf("=== Results (%d) ===\n", len(resp.Results))
	for i, result := range resp.Results {
		fmt.Printf("\n[%d] Score: %.4f\n", i+1, result.Score)
		fmt.Printf("Content: %s\n", truncate(result.Content, 200))
		if result.Title != "" {
			fmt.Printf("Title: %s\n", result.Title)
		}
	}

	// Print metadata
	fmt.Printf("\n=== Metadata ===\n")
	metaBytes, _ := json.MarshalIndent(resp.RetrievalMeta, "", "  ")
	fmt.Printf("%s\n", string(metaBytes))

	return nil
}

func loadRAGConfig(configPath string) (*compRag.RAGSpec, error) {
	loader := appconfig.NewLoader("config.yaml")
	componentCfg, err := loader.LoadComponent(configPath)
	if err != nil {
		return nil, err
	}
	if componentCfg.Type != "rag" {
		return nil, fmt.Errorf("component type must be rag, got %s", componentCfg.Type)
	}

	var cfg compRag.RAGSpec
	if err := componentCfg.Spec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode rag config: %w", err)
	}

	return &cfg, nil
}

func buildRAGSystem(g *genkit.Genkit, cfg *compRag.RAGSpec) (*compRag.RAG, error) {
	if g == nil {
		return nil, fmt.Errorf("genkit registry is nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("rag config is nil")
	}

	var specNode yaml.Node
	if err := specNode.Encode(cfg); err != nil {
		return nil, fmt.Errorf("failed to encode rag config: %w", err)
	}

	compRaw, err := compRag.RAGFactory(&specNode)
	if err != nil {
		return nil, fmt.Errorf("failed to create rag component: %w", err)
	}

	ragComp, ok := compRaw.(*compRag.RAGComponent)
	if !ok {
		return nil, fmt.Errorf("invalid rag component type: %T", compRaw)
	}

	rt := runtime.NewRuntime()
	rt.SetGenkitRegistry(g)
	if err := ragComp.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate rag component: %w", err)
	}
	if err := ragComp.Init(rt); err != nil {
		return nil, fmt.Errorf("failed to init rag component: %w", err)
	}
	if ragComp.Rag == nil {
		return nil, fmt.Errorf("rag system is not initialized")
	}

	return ragComp.Rag, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
