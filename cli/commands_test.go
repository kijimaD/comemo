package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCreateApp tests the CreateApp function
func TestCreateApp(t *testing.T) {
	app := CreateApp()

	assert.NotNil(t, app)
	assert.Equal(t, "comemo", app.Name)
	assert.Equal(t, "Go repository commit explanation generator", app.Usage)
	assert.Equal(t, "1.0.0", app.Version)

	// Check that global flags exist
	assert.NotEmpty(t, app.Flags)

	// Check that commands exist
	assert.NotEmpty(t, app.Commands)

	// Verify specific commands exist
	commandNames := make(map[string]bool)
	for _, cmd := range app.Commands {
		commandNames[cmd.Name] = true
	}

	expectedCommands := []string{"collect", "generate", "execute", "verify", "all", "status"}
	for _, cmdName := range expectedCommands {
		assert.True(t, commandNames[cmdName], "Command %s should exist", cmdName)
	}
}
