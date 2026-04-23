package typed

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompressDecompress_RoundTrip(t *testing.T) {
	ValueCompressionEnabled = true
	defer func() { ValueCompressionEnabled = false }()

	original := []byte(strings.Repeat(`{"kind":"Pod","metadata":{"name":"test-pod","namespace":"default"},"spec":{"containers":[{"name":"app","image":"nginx:latest"}]}}`, 5))

	compressed, err := CompressValue(original)
	assert.NoError(t, err)
	assert.True(t, IsCompressed(compressed))
	assert.True(t, len(compressed) < len(original), "compressed should be smaller for non-trivial payloads")

	decompressed, err := DecompressValue(compressed)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(original, decompressed))
}

func TestDecompress_UncompressedData(t *testing.T) {
	raw := []byte(`{"kind":"Pod","metadata":{"name":"test"}}`)

	result, err := DecompressValue(raw)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(raw, result), "uncompressed data should pass through unchanged")
}

func TestCompress_DisabledPassesThrough(t *testing.T) {
	ValueCompressionEnabled = false

	original := []byte(`{"kind":"Pod"}`)
	result, err := CompressValue(original)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(original, result), "should return original when compression disabled")
	assert.False(t, IsCompressed(result))
}

func TestCompressDecompress_LargePayload(t *testing.T) {
	ValueCompressionEnabled = true
	defer func() { ValueCompressionEnabled = false }()

	large := []byte(strings.Repeat(`{"kind":"Pod","metadata":{"name":"test-pod","namespace":"default","labels":{"app":"test","version":"v1"}}}`, 100))

	compressed, err := CompressValue(large)
	assert.NoError(t, err)
	ratio := float64(len(compressed)) / float64(len(large))
	t.Logf("Original: %d bytes, Compressed: %d bytes, Ratio: %.2f%%", len(large), len(compressed), ratio*100)
	assert.True(t, ratio < 0.15, "highly repetitive JSON should compress to <15%% of original")

	decompressed, err := DecompressValue(compressed)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(large, decompressed))
}

func TestDecompress_SmallData(t *testing.T) {
	tiny := []byte{0x08, 0x01}
	result, err := DecompressValue(tiny)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal(tiny, result), "small data should pass through")
}

func TestDecompress_EmptyData(t *testing.T) {
	result, err := DecompressValue([]byte{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestIsCompressed(t *testing.T) {
	assert.False(t, IsCompressed([]byte{}))
	assert.False(t, IsCompressed([]byte{0x08, 0x01}))
	assert.False(t, IsCompressed([]byte{0x28, 0xB5, 0x2F}))
	assert.True(t, IsCompressed([]byte{0x28, 0xB5, 0x2F, 0xFD, 0x00}))
}

func BenchmarkCompressValue(b *testing.B) {
	ValueCompressionEnabled = true
	defer func() { ValueCompressionEnabled = false }()

	payload := []byte(strings.Repeat(`{"kind":"Pod","metadata":{"name":"test-pod","namespace":"default"}}`, 50))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CompressValue(payload)
	}
}

func BenchmarkDecompressValue(b *testing.B) {
	ValueCompressionEnabled = true
	defer func() { ValueCompressionEnabled = false }()

	payload := []byte(strings.Repeat(`{"kind":"Pod","metadata":{"name":"test-pod","namespace":"default"}}`, 50))
	compressed, _ := CompressValue(payload)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecompressValue(compressed)
	}
}
