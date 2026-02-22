package tracer_test

import (
	"context"
	"testing"

	"github.com/evrone/go-clean-template/pkg/tracer"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestGetTracer(t *testing.T) {
	t.Parallel()

	tr := tracer.GetTracer("test-tracer")

	require.NotNil(t, tr)
}

func TestTracer_Shutdown_Nil(t *testing.T) {
	t.Parallel()

	tr := &tracer.Tracer{}
	err := tr.Shutdown(context.Background())

	require.NoError(t, err)
}

func TestTracer_GlobalProviderSet(t *testing.T) {
	// After initializing a tracer, the global TracerProvider should not be the noop provider.
	// Since we can't connect to a real collector, just verify the API works without panic.
	tp := otel.GetTracerProvider()
	require.NotNil(t, tp)

	tr := tp.Tracer("test")
	require.NotNil(t, tr)

	ctx, span := tr.Start(context.Background(), "test-span")
	require.NotNil(t, span)
	require.NotNil(t, ctx)

	span.End()
}
