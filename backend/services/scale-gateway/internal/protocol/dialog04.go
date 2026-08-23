package protocol

import "fmt"

// Dialog04 field widths, inferred from the vendor's example frames:
//
//	Request  (set price): STX ppppppp ETX BCC              — 7-digit price, cents
//	Response (transaction): STX sss wwww pppppp ETX BCC    — 3-digit status,
//	                                                          4-digit weight (grams),
//	                                                          6-digit price (cents)
//
// Dialog 04 appears to be a wider-field variant of Dialog 02 carrying the same
// values with more headroom (larger prices, plus a longer status/prefix field).
// As with Dialog 02, this layout is inferred from four example frames, not an
// official spec, and is unverified against real hardware.
const (
	dialog04PriceWidth  = 7
	dialog04StatusWidth = 3
	dialog04WeightWidth = 4
	dialog04RespPriceW  = 6
)

// Dialog04Codec implements Codec for Bizerba's Dialog 04 message format.
type Dialog04Codec struct{}

func (Dialog04Codec) Name() string { return "dialog04" }

func (Dialog04Codec) EncodeSetPrice(pricePerKgCents int) ([]byte, error) {
	field, err := formatDigits(pricePerKgCents, dialog04PriceWidth)
	if err != nil {
		return nil, fmt.Errorf("dialog04: encode price: %w", err)
	}
	return BuildFrame([]byte(field)), nil
}

func (Dialog04Codec) DecodeTransactionResponse(frame []byte) (TransactionResult, error) {
	payload, err := ParseFrame(frame)
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog04: %w", err)
	}

	fields, err := splitFields(payload, dialog04StatusWidth, dialog04WeightWidth, dialog04RespPriceW)
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog04: %w", err)
	}

	weight, err := parseDigits(fields[1])
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog04: weight field: %w", err)
	}
	price, err := parseDigits(fields[2])
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog04: price field: %w", err)
	}

	return TransactionResult{
		StatusCode:  fields[0],
		WeightGrams: weight,
		PriceCents:  price,
		RawFrame:    append([]byte(nil), frame...),
	}, nil
}
