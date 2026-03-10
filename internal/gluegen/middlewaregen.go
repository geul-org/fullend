package gluegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// claimToVerifyField maps a JWT claim key to the corresponding pkg/auth.VerifyTokenResponse field.
var claimToVerifyField = map[string]struct {
	Field  string // VerifyTokenResponse field name
	GoType string // Go type
}{
	"user_id": {Field: "UserID", GoType: "int64"},
	"email":   {Field: "Email", GoType: "string"},
	"role":    {Field: "Role", GoType: "string"},
}

// generateMiddleware creates internal/middleware/bearerauth.go with claims-based CurrentUser mapping.
func generateMiddleware(intDir, modulePath string, claims map[string]string) error {
	mwDir := filepath.Join(intDir, "middleware")
	if err := os.MkdirAll(mwDir, 0755); err != nil {
		return err
	}

	// Sort fields for deterministic output.
	var fields []string
	for field := range claims {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	// Build CurrentUser field assignments from VerifyTokenResponse.
	var assignments []string
	for _, field := range fields {
		claimKey := claims[field]
		mapping, ok := claimToVerifyField[claimKey]
		if !ok {
			// Unknown claim key — skip with comment.
			assignments = append(assignments, fmt.Sprintf("\t\t\t// %s: unknown claim key %q", field, claimKey))
			continue
		}
		assignments = append(assignments, fmt.Sprintf("\t\t\t%s: out.%s,", field, mapping.Field))
	}
	assignBlock := strings.Join(assignments, "\n")

	src := fmt.Sprintf(`package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/geul-org/fullend/pkg/auth"
	"%s/internal/model"
)

// BearerAuth returns a gin middleware that validates the Authorization header.
// Requests without a valid Bearer token are rejected with 401.
func BearerAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		out, err := auth.VerifyToken(auth.VerifyTokenRequest{Token: token, Secret: secret})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("currentUser", &model.CurrentUser{
%s
		})
		c.Next()
	}
}
`, modulePath, assignBlock)

	return os.WriteFile(filepath.Join(mwDir, "bearerauth.go"), []byte(src), 0644)
}
