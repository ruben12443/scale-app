package protocol

import "testing"

func TestDialog04EncodeSetPrice(t *testing.T) {
	codec := Dialog04Codec{}
	frame, err := codec.EncodeSetPrice(1499)
	if err != nil {
		t.Fatalf("EncodeSetPrice(1499) returned error: %v", err)
	}
	want := BuildFrame([]byte("0001499"))
	if string(frame) != string(want) {
		t.Fatalf("EncodeSetPrice(1499) = % X, want % X", frame, want)
	}
}

func TestDialog04EncodeSetPriceRejectsOutOfRange(t *testing.T) {
	codec := Dialog04Codec{}
	if _, err := codec.EncodeSetPrice(10000000); err == nil {
		t.Fatal("expected error for a price that does not fit in 7 digits, got nil")
	}
	if _, err := codec.EncodeSetPrice(-1); err == nil {
		t.Fatal("expected error for a negative price, got nil")
	}
}

func TestDialog04DecodeTransactionResponse(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("1001250001874"))

	got, err := codec.DecodeTransactionResponse(frame)
	if err != nil {
		t.Fatalf("DecodeTransactionResponse returned error: %v", err)
	}

	want := TransactionResult{StatusCode: "100", WeightGrams: 1250, PriceCents: 1874, RawFrame: frame}
	if got.StatusCode != want.StatusCode || got.WeightGrams != want.WeightGrams || got.PriceCents != want.PriceCents {
		t.Fatalf("DecodeTransactionResponse = %+v, want %+v", got, want)
	}
}

func TestDialog04DecodeTransactionResponseRejectsWrongLength(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("12345")) // far short of the expected 13
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for payload with wrong length, got nil")
	}
}

func TestDialog04DecodeTransactionResponseRejectsBadFrame(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("1001250001874"))
	frame[len(frame)-1] ^= 0xFF // corrupt BCC
	if _, err := codec.DecodeTransactionResponse(frame); err == nil {
		t.Fatal("expected error for frame with bad BCC, got nil")
	}
}

func TestDialog04Name(t *testing.T) {
	if got := (Dialog04Codec{}).Name(); got != "dialog04" {
		t.Fatalf("Name() = %q, want %q", got, "dialog04")
	}
}

func TestDialog04DecodeSetPriceRequest(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("0001499"))

	price, err := codec.DecodeSetPriceRequest(frame)
	if err != nil {
		t.Fatalf("DecodeSetPriceRequest returned error: %v", err)
	}
	if price != 1499 {
		t.Fatalf("price = %d, want 1499", price)
	}
}

func TestDialog04DecodeSetPriceRequestRejectsWrongLength(t *testing.T) {
	codec := Dialog04Codec{}
	frame := BuildFrame([]byte("1499")) // too short
	if _, err := codec.DecodeSetPriceRequest(frame); err == nil {
		t.Fatal("expected error for payload with wrong length, got nil")
	}
}

func TestDialog04EncodeSetPriceThenDecodeSetPriceRequestRoundTrips(t *testing.T) {
	codec := Dialog04Codec{}
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

func TestDialog04EncodeTransactionResponseThenDecodeRoundTrips(t *testing.T) {
	codec := Dialog04Codec{}
	frame, err := codec.EncodeTransactionResponse("100", 1250, 1874)
	if err != nil {
		t.Fatalf("EncodeTransactionResponse returned error: %v", err)
	}

	want := BuildFrame([]byte("1001250001874"))
	if string(frame) != string(want) {
		t.Fatalf("EncodeTransactionResponse = % X, want % X", frame, want)
	}

	result, err := codec.DecodeTransactionResponse(frame)
	if err != nil {
		t.Fatalf("DecodeTransactionResponse returned error: %v", err)
	}
	if result.StatusCode != "100" || result.WeightGrams != 1250 || result.PriceCents != 1874 {
		t.Fatalf("DecodeTransactionResponse result = %+v, want status=100 weight=1250 price=1874", result)
	}
}

func TestDialog04EncodeTransactionResponseRejectsBadStatusWidth(t *testing.T) {
	codec := Dialog04Codec{}
	if _, err := codec.EncodeTransactionResponse("1", 1250, 1874); err == nil {
		t.Fatal("expected error for a status code shorter than 3 characters, got nil")
	}
}

func TestDialog04EncodeTransactionResponseRejectsOutOfRangeFields(t *testing.T) {
	codec := Dialog04Codec{}
	if _, err := codec.EncodeTransactionResponse("100", 10000, 1874); err == nil {
		t.Fatal("expected error for weight that doesn't fit in 4 digits, got nil")
	}
	if _, err := codec.EncodeTransactionResponse("100", 1250, 1000000); err == nil {
		t.Fatal("expected error for price that doesn't fit in 6 digits, got nil")
	}
}
