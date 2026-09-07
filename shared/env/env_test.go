package env

import (
	"legnaris/shared/logger/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewEnvDefaultValues(t *testing.T) {
	structuredLogger := mocks.NewMockStructuredLogger(t)
	structuredLogger.On("Warn", mock.Anything, "env variable [%s] not found using default value").Twice()

	newEnv := NewEnv(structuredLogger)
	assert.NotNil(t, newEnv, "NewEnv should not return nil")
	assert.NotNil(t, newEnv.logger, "Logger should not be nil")
	assert.Equal(t, defaultPort, newEnv.Variables.PORT, "Default PORT should be set correctly")
	assert.Equal(t, defaultAppName, newEnv.Variables.APP_NAME, "Default APP_NAME should be set correctly")
}

func TestNewEnvWithCustomValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("APP_NAME", "customApp")

	newEnv := NewEnv(mocks.NewMockStructuredLogger(t))
	assert.NotNil(t, newEnv, "NewEnv should not return nil")
	assert.NotNil(t, newEnv.logger, "Logger should not be nil")
	assert.Equal(t, "9090", newEnv.Variables.PORT, "Custom PORT should be set correctly")
	assert.Equal(t, "customApp", newEnv.Variables.APP_NAME, "Custom APP_NAME should be set correctly")
}
