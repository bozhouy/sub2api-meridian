package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoundedDiagnosticWriterCancelsOversize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &boundedDiagnosticWriter{header: make(http.Header), cancel: cancel}
	_, err := w.Write(make([]byte, 2<<20))
	require.NoError(t, err)
	_, err = w.Write([]byte("x"))
	require.Error(t, err)
	require.True(t, w.exceeded)
	require.Error(t, ctx.Err())
	require.Equal(t, 2<<20, w.body.Len())
}

func TestDiagnosticRequiresCompletedResponse(t *testing.T) {
	for _, test := range []struct {
		raw      string
		complete bool
	}{
		{`{"status":"completed","output_text":"1 2 3"}`, true},
		{`{"status":"incomplete","output_text":"1 2 3"}`, false},
		{"data: {\"type\":\"response.output_text.delta\",\"delta\":\"1 2\"}\n", false},
		{"data: {\"type\":\"response.output_text.delta\",\"delta\":\"1 2\"}\ndata: {\"type\":\"response.completed\"}\n", true},
	} {
		_, complete := codexDiagnosticOutput([]byte(test.raw))
		require.Equal(t, test.complete, complete)
	}
}
