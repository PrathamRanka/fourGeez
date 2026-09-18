package recipes

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/fourgeez/agentpay/internal/integrations/stacks"
)

// TestJavaScriptFixturesHaveMaintainedRecipes verifies every advertised JS stack.
func TestJavaScriptFixturesHaveMaintainedRecipes(t *testing.T) {
	t.Parallel()

	fixtures := readJavaScriptFixtures(t)
	detector := stacks.NewService()
	service := NewService()
	for _, fixture := range fixtures {
		t.Run(string(fixture.Stack), func(t *testing.T) {
			detections, err := detector.Detect(fixture.Evidence)
			if err != nil {
				t.Fatal(err)
			}
			if !hasMaintainedDetection(detections, fixture.Stack) {
				t.Fatalf("detections = %#v", detections)
			}
			recipe, err := service.Recipe(fixture.Stack)
			if err != nil {
				t.Fatal(err)
			}
			if recipe.Language != LanguageNode ||
				recipe.VerificationPackage != "@agentpay/verify-node" ||
				recipe.VerificationAdapter != fixture.Adapter ||
				!containsFragment(recipe.StorefrontFiles, fixture.MetadataFragment) {
				t.Fatalf("recipe = %#v", recipe)
			}
			if err := Validate(recipe); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

// TestRecipeRejectsUnavailableStacks verifies support advertising stays closed.
func TestRecipeRejectsUnavailableStacks(t *testing.T) {
	t.Parallel()

	if _, err := NewService().Recipe(stacks.StackASPNetCore); err != ErrRecipeUnavailable {
		t.Fatalf("Recipe() error = %v", err)
	}
}

type javaScriptFixture struct {
	Stack            stacks.Stack      `json:"stack"`
	Evidence         map[string]string `json:"evidence"`
	Adapter          string            `json:"adapter"`
	MetadataFragment string            `json:"metadataFragment"`
}

// readJavaScriptFixtures loads committed stack evidence fixtures.
func readJavaScriptFixtures(t *testing.T) []javaScriptFixture {
	t.Helper()

	encoded, err := os.ReadFile("testdata/javascript.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []javaScriptFixture
	if err := json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

// hasMaintainedDetection reports whether one stack is safely advertised.
func hasMaintainedDetection(detections []stacks.Detection, stack stacks.Stack) bool {
	for _, detection := range detections {
		if detection.Stack == stack && detection.Tier == stacks.SupportTierMaintained {
			return true
		}
	}
	return false
}

// containsFragment reports whether any recipe path contains a required marker.
func containsFragment(paths []string, fragment string) bool {
	for _, path := range paths {
		if strings.Contains(path, fragment) {
			return true
		}
	}
	return false
}
