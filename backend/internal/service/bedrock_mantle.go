package service

import (
	"fmt"
	"strings"
)

// bedrockMantleAuthMode marks an account (anthropic/bedrock or openai/apikey) as
// pointing at an Amazon Bedrock Mantle endpoint — a native Anthropic/OpenAI
// protocol endpoint reached with a plain API key. It rides the native
// passthrough paths, NOT the SigV4/InvokeModel path.
const bedrockMantleAuthMode = "bedrock_mantle"

// defaultBedrockMantleRegion is the only region Mantle currently serves.
const defaultBedrockMantleRegion = "eu-north-1"

func bedrockMantleRegion(a *Account) string {
	if a == nil {
		return defaultBedrockMantleRegion
	}
	if r := strings.TrimSpace(a.GetCredential("aws_region")); r != "" {
		return r
	}
	if r := strings.TrimSpace(a.GetCredential("region")); r != "" {
		return r
	}
	return defaultBedrockMantleRegion
}

func BuildBedrockMantleBaseURL(region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		region = defaultBedrockMantleRegion
	}
	return fmt.Sprintf("https://bedrock-mantle.%s.api.aws", region)
}

func BuildBedrockMantleMessagesURL(region string) string {
	return BuildBedrockMantleBaseURL(region) + "/v1/messages"
}

func BuildBedrockMantleChatCompletionsURL(region string) string {
	return BuildBedrockMantleBaseURL(region) + "/v1/chat/completions"
}

// ResolveBedrockMantleModelID maps a requested model to Mantle's global model ID
// form (e.g. claude-opus-4-8 -> global.anthropic.claude-opus-4-8-v1) by reusing
// the existing Bedrock normalization and forcing the "global" region prefix.
// Returns ok=false for models that are not recognized Bedrock/Claude models, so
// callers can fall back to verbatim passthrough.
func ResolveBedrockMantleModelID(account *Account, requestedModel string) (string, bool) {
	if account == nil {
		return "", false
	}
	mapped := account.GetMappedModel(requestedModel)
	modelID, shouldAdjustRegion, ok := normalizeBedrockModelID(mapped)
	if !ok {
		return "", false
	}
	if shouldAdjustRegion {
		modelID = AdjustBedrockModelRegionPrefix(modelID, "global")
	}
	return modelID, true
}
