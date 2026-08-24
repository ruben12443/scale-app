package protocol

import "fmt"

// Dialog02 field widths, inferred from the vendor's example frames:
//
//	Request  (set price): STX ppppp ETX BCC          — 5-digit price, cents
//	Response (transaction): STX s wwww pppp ETX BCC  — 1-digit status,
//	                                                    4-digit weight (grams),
//	                                                    4-digit price (cents)
//
// This layout is inferred from four example frames provided by the user, not
// from an official protocol specification, and has not been validated against
// real hardware. In particular, the meaning of the status digit is unknown.
const (
	dialog02PriceWidth  = 5
	dialog02StatusWidth = 1
	dialog02WeightWidth = 4
	dialog02RespPriceW  = 4
)

// Dialog02Codec implements Codec for Bizerba's Dialog 02 message format.
type Dialog02Codec struct{}

func (Dialog02Codec) Name() string { return "dialog02" }

func (Dialog02Codec) EncodeSetPrice(pricePerKgCents int) ([]byte, error) {
	field, err := formatDigits(pricePerKgCents, dialog02PriceWidth)
	if err != nil {
		return nil, fmt.Errorf("dialog02: encode price: %w", err)
	}
	return BuildFrame([]byte(field)), nil
}

func (Dialog02Codec) DecodeTransactionResponse(frame []byte) (TransactionResult, error) {
	payload, err := ParseFrame(frame)
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog02: %w", err)
	}

	fields, err := splitFields(payload, dialog02StatusWidth, dialog02WeightWidth, dialog02RespPriceW)
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog02: %w", err)
	}

	weight, err := parseDigits(fields[1])
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog02: weight field: %w", err)
	}
	price, err := parseDigits(fields[2])
	if err != nil {
		return TransactionResult{}, fmt.Errorf("dialog02: price field: %w", err)
	}

	return TransactionResult{
		StatusCode:  fields[0],
		WeightGrams: weight,
		PriceCents:  price,
		RawFrame:    append([]byte(nil), frame...),
	}, nil
}

func (Dialog02Codec) DecodeSetPriceRequest(frame []byte) (int, error) {
	payload, err := ParseFrame(frame)
	if err != nil {
		return 0, fmt.Errorf("dialog02: %w", err)
	}
	if len(payload) != dialog02PriceWidth {
		return 0, fmt.Errorf("dialog02: set-price payload length %d, want %d", len(payload), dialog02PriceWidth)
	}
	price, err := parseDigits(string(payload))
	if err != nil {
		return 0, fmt.Errorf("dialog02: price field: %w", err)
	}
	return price, nil
}

func (Dialog02Codec) EncodeTransactionResponse(statusCode string, weightGrams, priceCents int) ([]byte, error) {
	if len(statusCode) != dialog02StatusWidth {
		return nil, fmt.Errorf("dialog02: status code must be exactly %d character(s), got %q", dialog02StatusWidth, statusCode)
	}
	weightField, err := formatDigits(weightGrams, dialog02WeightWidth)
	if err != nil {
		return nil, fmt.Errorf("dialog02: encode weight: %w", err)
	}
	priceField, err := formatDigits(priceCents, dialog02RespPriceW)
	if err != nil {
		return nil, fmt.Errorf("dialog02: encode price: %w", err)
	}
	payload := statusCode + weightField + priceField
	return BuildFrame([]byte(payload)), nil
}
