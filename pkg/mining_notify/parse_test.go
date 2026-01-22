package mining_notify_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adamdecaf/pool_checkr/pkg/mining_notify"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	cases := []struct {
		inputFilepath string
		expected      *mining_notify.ParsedNotify
	}{
		{
			inputFilepath: filepath.Join("testdata", "256-foundation.txt"),
			expected: &mining_notify.ParsedNotify{
				JobID:     "188d18305556824a",
				PrevHash:  "0000000000000000dad70100cd17f439982b0ab99b5452e0e3092d7644722fc0",
				Height:    933395,
				ScriptSig: "03133e0e010004b050726904c0e425150c0e6879647261706f6f6c2f32353666ffffffff020097b012000000",
				CoinbaseOuts: []mining_notify.CoinbaseOutput{
					{
						ValueSatoshis: 313562880,
						ValueBTC:      3.1356288,
						Type:          "P2WPKH",
						Address:       "bc1qce93hy5rhg02s6aeu7mfdvxg76x66pqqtrvzs3"},
					{
						ValueSatoshis: 0,
						ValueBTC:      0,
						Type:          "OP_RETURN",
						Address:       "(Null Data)",
					},
				},
				Version:     "20000000",
				NBits:       "1701ebf2",
				NTime:       "697250b0",
				NTimeParsed: time.Date(2026, time.January, 22, 16, 30, 40, 0, time.UTC),
				CleanJobs:   false,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.inputFilepath, func(t *testing.T) {
			bs, err := os.ReadFile(tc.inputFilepath)
			require.NoError(t, err)

			got, err := mining_notify.Parse(string(bs))
			require.NoError(t, err)
			require.Equal(t, tc.expected, got)
		})
	}
}
