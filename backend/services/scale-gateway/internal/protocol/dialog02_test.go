package protocol

import "testing"

func TestDialog02EncodeSetPrice(t *testing.T) {
	codec := Dialog02Codec{}
	frame, err := codec.EncodeSetPrice(1499)
	if err != nil {
		t.Fatalf("EncodeSetPrice(1499) returned error: %v", err)
	}
	want := BuildFrame([]byte("01499"))
	if string(frame) != string(want) {
		t.Fatalf("EncodeSetPrice(1499) = % X, want % X", frame, want)
	}
}

func TestDialog02EncodeSetPriceRejectsOutOfRange(t *testing.T) {
	codec := Dialog02Codec{}
	if _, err := codec.EncodeSetPrice(100000); err == nil {
		t.Fatal("expected error for a price that does not fit in 5 digits, got nil")
	}
	if _, err := codec.EncodeSetPrice(-1); err == nil {
		t.Fatal("expected error for a negative price, got nil")
	}
}

func TestDialog02DecodeTransactionResponse(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("112501874"))

	got, err := codec.DecodeTransactionResponse(frame)
	if err != nil {
		t.Fatalf("DecodeTransactionResponse returned error: %v", err)
	}

	want := TransactionResult{StatusCode: "1", WeightGrams: 1250, PriceCents: 1874, RawFrame: frame}
	if got.StatusCode != want.StatusCode || got.WeightGrams != want.WeightGrams || got.PriceCents != want.PriceCents {
		t.Fatalf("DecodeTransactionResponse = %+v, want %+v", got, want)
	}
}

func TestDialog02DecodeTransactionResponseRejectsWrongLength(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("1234")) // one digit short of the expected 9
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for payload with wrong length, got nil")
	}
}

func TestDialog02DecodeTransactionResponseRejectsBadFrame(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("112501874"))
	frame[len(frame)-1] ^= 0xFF // corrupt BCC
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for frame with bad BCC, got nil")
	}
}

func TestDialog02Name(t *testing.T) {
	if got := (Dialog02Codec{}).Name(); got != "dialog02" {
		t.Fatalf("Name() = %q, want %q", got, "dialog02")
	}
}

func TestDialog02DecodeSetPriceRequest(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("01499"))

	price, err := codec.DecodeSetPriceRequest(frame)
	if err != nil {
		t.Fatalf("DecodeSetPriceRequest returned error: %v", err)
	}
	if price != 1499 {
		t.Fatalf("price = %d, want 1499", price)
	}
}

func TestDialog02DecodeSetPriceRequestRejectsWrongLength(t *testing.T) {
	codec := Dialog02Codec{}
	frame := BuildFrame([]byte("149")) // too short
	if _, err := codec.DecodeSetPriceRequest(frame); err == nil {
		t.Fatal("expected error for payload with wrong length, got nil")
	}
}

func TestDialog02EncodeSetPriceThenDecodeSetPriceRequestRoundTrips(t *testing.T) {
	codec := Dialog02Codec{}
	frame, err := codec.EncodeSetPrice(1499)
	if err != nil {
		t.Fatalf("EncodeSetPrice returned error: %v", err)
	}
	price, err := codec.DecodeSetPriceRequest(frame)
	if err != nil {
		t.Fatalf("DecodeSetPriceRequest returned error: %v", err)
	}
	if price != 1499 {
		t.Fatalf("price = %d, want 1499", price)
	}
}

func TestDialog02EncodeTransactionResponseThenDecodeRoundTrips(t *testing.T) {
	codec := Dialog02Codec{}
	frame, err := codec.EncodeTransactionResponse("1", 1250, 1874)
	if err != nil {
		t.Fatalf("EncodeTransactionResponse returned error: %v", err)
	}

	want := BuildFrame([]byte("112501874"))
	if string(frame) != string(want) {
		t.Fatalf("EncodeTransactionResponse = % X, want % X", frame, want)
	}

	result, err := codec.DecodeTransactionResponse(frame)
	if err != nil {
		t.Fatalf("DecodeTransactionResponse returned error: %v", err)
	}
	if result.StatusCode != "1" || result.WeightGrams != 1250 || result.PriceCents != 1874 {
		t.Fatalf("DecodeTransactionResponse result = %+v, want status=1 weight=1250 price=1874", result)
	}
}

func TestDialog02EncodeTransactionResponseRejectsBadStatusWidth(t *testing.T) {
	codec := Dialog02Codec{}
	if _, err := codec.EncodeTransactionResponse("12", 1250, 1874); err == nil {
		t.Fatal("expected error for a status code longer than 1 character, got nil")
	}
	if _, err := codec.EncodeTransactionResponse("", 1250, 1874); err == nil {
		t.Fatal("expected error for an empty status code, got nil")
	}
}

func TestDialog02EncodeTransactionResponseRejectsOutOfRangeFields(t *testing.T) {
	codec := Dialog02Codec{}
	if _, err := codec.EncodeTransactionResponse("1", 10000, 1874); err == nil {
		t.Fatal("expected error for weight that doesn't fit in 4 digits, got nil")
	}
	if _, err := codec.EncodeTransactionResponse("1", 1250, 10000); err == nil {
		t.Fatal("expected error for price that doesn't fit in 4 digits, got nil")
	}
}
