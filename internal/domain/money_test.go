package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
)

func TestNormalizeCurrency(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"uppercase", "USD", "USD", false},
		{"lowercase", "eur", "EUR", false},
		{"whitespace", "  gbp  ", "GBP", false},
		{"too short", "US", "", true},
		{"too long", "USDD", "", true},
		{"digits", "US1", "", true},
		{"empty", "", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.NormalizeCurrency(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				var derr *domain.Error
				require.ErrorAs(t, err, &derr)
				assert.Equal(t, domain.CodeValidation, derr.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFormatMinor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		amount   int64
		currency string
		want     string
	}{
		{12345, "USD", "123.45 USD"},
		{100, "USD", "1.00 USD"},
		{5, "EUR", "0.05 EUR"},
		{0, "GBP", "0.00 GBP"},
		{-2550, "USD", "-25.50 USD"},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.want, domain.FormatMinor(tc.amount, tc.currency))
	}
}
