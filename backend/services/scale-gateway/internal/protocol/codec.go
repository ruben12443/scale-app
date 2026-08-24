package protocol

// TransactionResult is the parsed result of a scale's response to a
// set-price-and-weigh request.
//
// StatusCode is the raw, undecoded status/prefix digits from the response
// (1 digit for Dialog 02, 3 digits for Dialog 04). Its exact meaning (e.g.
// success/error/pending codes) is not documented anywhere available to this
// implementation and has not been verified against real hardware — treat it
// as opaque diagnostic data until confirmed.
type TransactionResult struct {
	StatusCode  string
	WeightGrams int
	PriceCents  int
	RawFrame    []byte
}

// Codec encodes and decodes both directions of one Dialog protocol variant:
// the client side (EncodeSetPrice/DecodeTransactionResponse, used by
// driver.DialogTCPDriver talking to a real scale) and the server side
// (DecodeSetPriceRequest/EncodeTransactionResponse, used by a scale
// simulator standing in for one, e.g. cmd/fake-scale).
type Codec interface {
	// Name identifies the protocol variant, e.g. "dialog02" or "dialog04".
	Name() string
	// EncodeSetPrice builds a complete wire frame that sets the price per kg
	// (in cents) on the scale and requests a transaction.
	EncodeSetPrice(pricePerKgCents int) ([]byte, error)
	// DecodeTransactionResponse parses a complete wire frame received from the
	// scale into a TransactionResult.
	DecodeTransactionResponse(frame []byte) (TransactionResult, error)
	// DecodeSetPriceRequest parses a complete set-price wire frame (as sent
	// by EncodeSetPrice) and returns the price per kg, in cents.
	DecodeSetPriceRequest(frame []byte) (pricePerKgCents int, err error)
	// EncodeTransactionResponse builds a complete transaction wire frame (as
	// parsed by DecodeTransactionResponse) from a status code, weight, and
	// total price.
	EncodeTransactionResponse(statusCode string, weightGrams, priceCents int) ([]byte, error)
}
