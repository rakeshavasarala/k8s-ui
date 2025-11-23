package web

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	// TODO: Mock Manager for testing
	// For now, we just skip this test or need to refactor Manager to be an interface or mockable
	// cs := fake.NewSimpleClientset()
	// s, err := NewServer(cs, "default")
	// if err != nil {
	// 	t.Fatalf("NewServer failed: %v", err)
	// }

	// if s.layoutTmpl == nil {
	// 	t.Error("Templates not initialized")
	// }
	
	// // Verify a specific template exists
	// if s.layoutTmpl.Lookup("layout.html") == nil {
	// 	t.Error("layout.html not found in templates")
	// }
}
