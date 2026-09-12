package ctxlog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWithLogger_FromContext_RoundTrip(t *testing.T) {
	t.Parallel()

	expected := zap.NewExample().Sugar()
	ctx := WithLogger(context.Background(), expected)

	got := FromContext(ctx)

	assert.Same(t, expected, got)
}

func TestFromContext_NoLoggerAttached(t *testing.T) {
	t.Parallel()

	got := FromContext(context.Background())

	require.NotNil(t, got)
	assert.NotPanics(t, func() {
		got.Infow("no-op logger should swallow this")
	})
}

func TestWithLogger_DoesNotMutateParentContext(t *testing.T) {
	t.Parallel()

	parent := context.Background()
	child := WithLogger(parent, zap.NewExample().Sugar())

	assert.NotSame(t, FromContext(parent), FromContext(child))
}
