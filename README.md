# Legnaris

## Generate mocks

Mockery is pinned as a Go tool dependency. Generate mocks for the repository interfaces with:

```sh
make mocks
```

Generated mocks are stored in package-local `mocks` packages:

- `shared/env/mocks` contains `EnvVars`.
- `shared/logger/mocks` contains `StructuredLogger`.

Use the generated constructor and expecter API in tests:

```go
import (
    "testing"

    "github.com/stretchr/testify/mock"
    envmocks "legnaris/shared/env/mocks"
)

func TestConsumerUsesEnvironment(t *testing.T) {
    environment := envmocks.NewMockEnvVars(t)
    environment.EXPECT().Get("MODE").Return("test").Once()
    environment.EXPECT().Set(mock.Anything, mock.Anything).Maybe()

    // Pass environment to the code under test.
}
```

Regenerate mocks whenever an interface changes. The Mockery configuration is in `.mockery.yaml`.
