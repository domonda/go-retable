package exceltable

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadLocalFile(t *testing.T) {
	gotSheets, err := ReadLocalFile("/Users/erik/Downloads/conthaus/Kontodaten.xlsx", true)
	require.NoError(t, err)
	require.Len(t, gotSheets, 1)
}
